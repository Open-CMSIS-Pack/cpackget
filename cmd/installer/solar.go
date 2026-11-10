/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"bytes"
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	xmlencoding "encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"golang.org/x/sync/errgroup"
)

const solarEndpoint = "https://solar-search.api.keil.arm.com/"

type TargetPack struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (pack TargetPack) ID() string {
	return pack.Vendor + "::" + pack.Name + "@" + pack.Version
}

type TargetResult struct {
	Name     string
	Revision string
	Vendor   struct {
		Name string
	}
	Pack          TargetPack
	Devices       []TargetResult
	mountedDevice string
}

type solarTarget struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
	Vendor   struct {
		Name string `json:"name"`
	} `json:"vendor"`
	FamilyID string `json:"source_pack_slug"`
	Cmsis    *struct {
		FamilyID string `json:"pack_family_id"`
	} `json:"cmsis"`
}

type solarClient struct {
	endpoint string
	http     *http.Client
	progress func(int)
}

type solarSearchCache struct {
	families       map[string]solarFamily
	devices        map[string][]TargetResult
	mountedDevices bool
	depth          int
}

type solarFamily struct {
	pack    TargetPack
	pdsc    *xml.PdscXML
	devices map[xml.DeviceIdentity]bool
}

func (client solarClient) query(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("solar: %s", response.Status)
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("solar: %s", envelope.Errors[0].Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("solar returned no data")
	}
	return json.Unmarshal(envelope.Data, result)
}

func (client solarClient) latestPdsc(ctx context.Context, familyID string) (TargetPack, *xml.PdscXML, error) {
	var family struct {
		Pack *struct {
			Identity TargetPack `json:"cmsis_id"`
			Release  struct {
				Version string `json:"version"`
				URL     string `json:"url"`
			} `json:"current_release"`
		} `json:"pack"`
	}
	if err := client.query(ctx, `query($id:String!){pack:get_pack_family(id:$id){cmsis_id{vendor name} current_release{version url}}}`, map[string]interface{}{"id": familyID}, &family); err != nil {
		return TargetPack{}, nil, err
	}
	if family.Pack == nil {
		return TargetPack{}, nil, fmt.Errorf("solar returned no pack for %q", familyID)
	}
	pack := family.Pack.Identity
	pack.Version = family.Pack.Release.Version
	if !validTargetPack(pack) {
		return pack, nil, fmt.Errorf("solar returned an unresolved pack for %q", familyID)
	}
	base, err := url.Parse(family.Pack.Release.URL)
	if err != nil {
		return pack, nil, err
	}
	if base.Scheme != "https" && base.Scheme != "http" {
		return pack, nil, fmt.Errorf("invalid pack URL for %s", pack.ID())
	}
	location := base.ResolveReference(&url.URL{Path: pack.Vendor + "." + pack.Name + utils.PdscExtension})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return pack, nil, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return pack, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return pack, nil, fmt.Errorf("pdsc %s: %s", location, response.Status)
	}
	var pdsc xml.PdscXML
	if err := xmlencoding.NewDecoder(response.Body).Decode(&pdsc); err != nil {
		return pack, nil, err
	}
	if pdsc.Vendor != pack.Vendor || pdsc.Name != pack.Name || utils.SemverCompare(pdsc.LatestVersion(), pack.Version) != 0 {
		return pack, nil, fmt.Errorf("latest PDSC does not match %s", pack.ID())
	}
	pack.Version = pdsc.LatestVersion()
	return pack, &pdsc, nil
}

func validTargetPack(pack TargetPack) bool {
	return pack.Vendor != "" && pack.Name != "" && utils.SemverMajor(pack.Version) != ""
}

func (client solarClient) search(ctx context.Context, kind, pattern string, mountedDevices ...bool) ([]TargetResult, error) {
	return client.searchSelected(ctx, kind, pattern, len(mountedDevices) == 0 || mountedDevices[0], false)
}

func (client solarClient) searchSelected(ctx context.Context, kind, pattern string, mountedDevices, exact bool) ([]TargetResult, error) {
	cache := &solarSearchCache{
		families:       make(map[string]solarFamily),
		devices:        make(map[string][]TargetResult),
		mountedDevices: mountedDevices,
	}
	return client.searchCached(ctx, kind, pattern, cache, exact)
}

func (client solarClient) discover(ctx context.Context, kind, pattern string, progress func(int, int)) ([]solarTarget, error) {
	vendor, name, qualified := strings.Cut(pattern, "::")
	if qualified {
		if strings.TrimSpace(vendor) == "" || strings.TrimSpace(name) == "" || strings.Contains(name, "::") {
			return nil, fmt.Errorf("a vendor-qualified pattern must have the form Vendor::pattern")
		}
		pattern = name
	}
	revision, hasRevision := "", false
	if kind == "board" {
		pattern, revision, hasRevision = strings.Cut(pattern, ":")
		if hasRevision && strings.TrimSpace(revision) == "" {
			return nil, fmt.Errorf("a board revision must not be empty")
		}
	}
	if (kind != "device" && kind != "board") || strings.TrimSpace(pattern) == "" {
		return nil, fmt.Errorf("a device or board search pattern is required")
	}
	results := []solarTarget{}
	normalized := strings.ToLower(pattern)
	seen := make(map[string]bool)
	var progressLock sync.Mutex
	total, completed := 0, 0
	advance := func() {
		progressLock.Lock()
		defer progressLock.Unlock()
		completed++
		if progress != nil {
			progress(completed, total)
		}
	}
	for offset := 0; ; {
		var page struct {
			Search *struct {
				Results []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"results"`
				Metadata struct {
					Total int `json:"total"`
				} `json:"metadata"`
			} `json:"search"`
		}
		query := fmt.Sprintf(`query($q:String!,$offset:Int!){search:search_%ss(q:$q,offset:$offset,limit:100){metadata{total} results{id name}}}`, kind)
		if err := client.query(ctx, query, map[string]interface{}{"q": pattern, "offset": offset}, &page); err != nil {
			return nil, err
		}
		if page.Search == nil || (len(page.Search.Results) == 0 && offset < page.Search.Metadata.Total) {
			return nil, fmt.Errorf("solar returned incomplete search results")
		}
		if offset == 0 {
			total = page.Search.Metadata.Total
			if progress != nil {
				progress(0, total)
			}
		}
		fields := `name vendor{name} source_pack_slug`
		if kind == "board" {
			fields = `name revision vendor{name} cmsis{pack_family_id}`
		}
		details := make([]*solarTarget, len(page.Search.Results))
		group, detailContext := errgroup.WithContext(ctx)
		group.SetLimit(8)
		for position, match := range page.Search.Results {
			if seen[match.ID] {
				advance()
				continue
			}
			seen[match.ID] = true
			group.Go(func() error {
				var detail struct {
					Target *solarTarget `json:"target"`
				}
				if err := client.query(detailContext, fmt.Sprintf(`query($id:String!){target:get_%s(id:$id){%s}}`, kind, fields), map[string]interface{}{"id": match.ID}, &detail); err != nil {
					return err
				}
				if detail.Target == nil {
					return fmt.Errorf("solar returned no details for %q", match.Name)
				}
				details[position] = detail.Target
				advance()
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return nil, err
		}
		for _, detail := range details {
			if detail == nil {
				continue
			}
			entry := *detail
			if hasRevision && !strings.Contains(strings.ToLower(entry.Revision), strings.ToLower(revision)) {
				continue
			}
			if qualified && !strings.Contains(strings.ToLower(entry.Vendor.Name), strings.ToLower(vendor)) {
				continue
			}
			if !strings.Contains(strings.ToLower(entry.Name), normalized) && (qualified || !strings.Contains(strings.ToLower(entry.Vendor.Name), normalized)) {
				continue
			}
			results = append(results, entry)
		}
		offset += len(page.Search.Results)
		if offset >= page.Search.Metadata.Total {
			break
		}
	}
	return results, nil
}

func (client solarClient) searchCached(ctx context.Context, kind, pattern string, cache *solarSearchCache, exact bool) ([]TargetResult, error) {
	var progress func(int)
	if cache.depth == 0 {
		progress = client.progress
	}
	cache.depth++
	defer func() { cache.depth-- }()
	cacheKey := strings.ToLower(pattern)
	if kind == "device" && !exact {
		if results, ok := cache.devices[cacheKey]; ok {
			return results, nil
		}
	}
	discovered := 0
	var discoveryProgress func(int, int)
	if progress != nil {
		discoveryProgress = func(completed, total int) {
			discovered = completed
			progress(100 * completed / (2*total + 1))
		}
	}
	candidates, err := client.discover(ctx, kind, pattern, discoveryProgress)
	if err != nil {
		return nil, err
	}
	if exact {
		name := pattern
		if _, qualifiedName, qualified := strings.Cut(pattern, "::"); qualified {
			name = qualifiedName
		}
		if kind == "board" {
			name, _, _ = strings.Cut(name, ":")
		}
		var matches []solarTarget
		for _, candidate := range candidates {
			if strings.EqualFold(candidate.Name, name) {
				matches = append(matches, candidate)
			}
		}
		if len(matches) > 0 {
			candidates = matches
		}
	}
	pendingFamilies := make(map[string]bool)
	for _, candidate := range candidates {
		familyID := candidate.FamilyID
		if candidate.Cmsis != nil {
			familyID = candidate.Cmsis.FamilyID
		}
		if _, cached := cache.families[familyID]; familyID != "" && !cached {
			pendingFamilies[familyID] = true
		}
	}
	totalSteps := discovered + len(pendingFamilies) + 1
	completedSteps := discovered
	if progress != nil {
		progress(100 * completedSteps / totalSteps)
	}
	results := []TargetResult{}
	for _, candidate := range candidates {
		entry := TargetResult{Name: candidate.Name, Revision: candidate.Revision}
		entry.Vendor.Name = candidate.Vendor.Name
		familyID := candidate.FamilyID
		if candidate.Cmsis != nil {
			familyID = candidate.Cmsis.FamilyID
		}
		if familyID != "" {
			family, cached := cache.families[familyID]
			if !cached {
				pack, pdsc, err := client.latestPdsc(ctx, familyID)
				if err != nil {
					return nil, err
				}
				family = solarFamily{pack: pack, pdsc: pdsc, devices: make(map[xml.DeviceIdentity]bool)}
				for _, device := range pdsc.TargetDevices() {
					family.devices[device] = true
				}
				cache.families[familyID] = family
				completedSteps++
				if progress != nil {
					progress(100 * completedSteps / totalSteps)
				}
			}
			entry.Pack = family.pack
			present := false
			if kind == "device" {
				present = family.devices[xml.DeviceIdentity{Vendor: entry.Vendor.Name, Name: entry.Name}]
			} else {
				for _, board := range family.pdsc.Boards {
					if board.Name != entry.Name || board.Vendor != entry.Vendor.Name || board.Revision != entry.Revision {
						continue
					}
					present = true
					entry.Devices = nil
					if !cache.mountedDevices {
						break
					}
					for _, mounted := range board.MountedDevices {
						vendor := strings.SplitN(mounted.Vendor, ":", 2)[0]
						providers, err := client.searchCached(ctx, "device", vendor+"::"+mounted.Name, cache, false)
						if err != nil {
							return nil, err
						}
						exact := false
						for _, provider := range providers {
							if strings.EqualFold(provider.Vendor.Name, vendor) && strings.EqualFold(provider.Name, mounted.Name) {
								exact = true
								break
							}
						}
						resolved := false
						for _, provider := range providers {
							if !strings.EqualFold(provider.Vendor.Name, vendor) {
								continue
							}
							if strings.EqualFold(provider.Name, mounted.Name) || (!exact && strings.HasPrefix(strings.ToLower(provider.Name), strings.ToLower(mounted.Name))) {
								provider.mountedDevice = vendor + "::" + mounted.Name
								entry.Devices = append(entry.Devices, provider)
								resolved = true
							}
						}
						if !resolved {
							missing := TargetResult{Name: mounted.Name}
							missing.Vendor.Name = vendor
							entry.Devices = append(entry.Devices, missing)
						}
					}
					break
				}
			}
			if !present {
				continue
			}
		} else if kind == "device" {
			return nil, fmt.Errorf("solar returned no pack family for %q", entry.Name)
		}
		if kind == "board" && !validTargetPack(entry.Pack) {
			continue
		}
		results = append(results, entry)
	}
	sort.Slice(results, func(left, right int) bool {
		first, second := results[left], results[right]
		return cmp.Or(strings.Compare(first.Vendor.Name, second.Vendor.Name), strings.Compare(first.Name, second.Name),
			strings.Compare(first.Revision, second.Revision), strings.Compare(first.Pack.Vendor, second.Pack.Vendor),
			strings.Compare(first.Pack.Name, second.Pack.Name), strings.Compare(first.Pack.Version, second.Pack.Version)) < 0
	})
	if kind == "device" && !exact {
		cache.devices[cacheKey] = results
	}
	if progress != nil {
		progress(100)
	}
	return results, nil
}

func FindTargets(ctx context.Context, kind, pattern string, insecureSkipVerify bool, timeout int, mountedDevices ...bool) ([]TargetResult, error) {
	return findTargets(ctx, kind, pattern, insecureSkipVerify, timeout, len(mountedDevices) == 0 || mountedDevices[0], false)
}

func FindTargetsForInstallation(ctx context.Context, kind, pattern string, insecureSkipVerify bool, timeout int) ([]TargetResult, error) {
	return findTargets(ctx, kind, pattern, insecureSkipVerify, timeout, true, true)
}

func findTargets(ctx context.Context, kind, pattern string, insecureSkipVerify bool, timeout int, mountedDevices, exact bool) ([]TargetResult, error) {
	return FindTargetsWithProgress(ctx, kind, pattern, insecureSkipVerify, timeout, mountedDevices, exact, nil)
}

func FindTargetsWithProgress(ctx context.Context, kind, pattern string, insecureSkipVerify bool, timeout int, mountedDevices, installation bool, progress func(int)) ([]TargetResult, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if insecureSkipVerify {
		transport.TLSClientConfig.InsecureSkipVerify = true // #nosec G402
	}
	defer transport.CloseIdleConnections()
	client := solarClient{endpoint: solarEndpoint, http: &http.Client{Transport: transport, Timeout: time.Duration(timeout) * time.Second}, progress: progress}
	return client.searchSelected(ctx, kind, pattern, mountedDevices, installation)
}

func ResolveTargetPacks(kind string, results []TargetResult, patterns ...string) ([]string, error) {
	pattern := ""
	if len(patterns) > 0 {
		pattern = patterns[0]
	}
	return resolveTargetPacks(kind, results, pattern)
}

func resolveTargetPacks(kind string, results []TargetResult, pattern string) ([]string, error) {
	if pattern != "" {
		vendor, name, qualified := strings.Cut(pattern, "::")
		if !qualified {
			name = vendor
		}
		if kind == "board" {
			boardName, revision, hasRevision := strings.Cut(name, ":")
			name = boardName
			if hasRevision {
				filtered := []TargetResult{}
				for _, result := range results {
					if strings.Contains(strings.ToLower(result.Revision), strings.ToLower(revision)) {
						filtered = append(filtered, result)
					}
				}
				results = filtered
			}
		}
		var exact []TargetResult
		for _, result := range results {
			if strings.EqualFold(result.Name, name) && (!qualified || strings.Contains(strings.ToLower(result.Vendor.Name), strings.ToLower(vendor))) {
				exact = append(exact, result)
			}
		}
		if len(exact) > 0 {
			results = exact
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no matching targets found")
	}
	var selected []string
	candidates := make(map[string]bool)
	ambiguous := false
	for _, result := range results {
		name := result.Vendor.Name + "::" + result.Name
		if result.Revision != "" {
			name += ":" + result.Revision
		}
		candidates[name] = true
		packs := make(map[string]bool)
		if !validTargetPack(result.Pack) {
			if kind == "board" {
				return nil, fmt.Errorf("cannot install %s: board pack provider is unresolved", name)
			}
			ambiguous = true
		} else {
			packs[result.Pack.ID()] = true
		}
		if kind == "board" {
			providers := make(map[string]string)
			for _, device := range result.Devices {
				if !validTargetPack(device.Pack) {
					ambiguous = true
					continue
				}
				packs[device.Pack.ID()] = true
				identity := device.Vendor.Name + "::" + device.Name
				if device.mountedDevice != "" {
					identity = device.mountedDevice
				}
				if previous, exists := providers[identity]; exists && previous != device.Pack.ID() {
					ambiguous = true
				}
				providers[identity] = device.Pack.ID()
			}
		}
		current := []string{}
		for pack := range packs {
			current = append(current, pack)
		}
		sort.Strings(current)
		if selected == nil {
			selected = current
		} else if !slices.Equal(selected, current) {
			ambiguous = true
		}
	}
	if ambiguous {
		choices := []string{}
		for pack := range candidates {
			choices = append(choices, pack)
		}
		sort.Strings(choices)
		return nil, fmt.Errorf("%s selection is ambiguous or unresolved; specify a %s explicitly:\n%s", kind, kind, strings.Join(choices, "\n"))
	}
	return selected, nil
}

func TargetPackStatus(root string, pack TargetPack) (string, error) {
	return NewTargetStatusLookup(root).Status(pack)
}

type TargetStatusLookup struct {
	root   string
	loaded bool
	tags   []xml.PdscTag
	err    error
}

func NewTargetStatusLookup(root string) *TargetStatusLookup {
	return &TargetStatusLookup{root: root}
}

func (lookup *TargetStatusLookup) Status(pack TargetPack) (string, error) {
	status := ""
	path := filepath.Join(lookup.root, pack.Vendor, pack.Name, utils.SemverStripMeta(pack.Version))
	if utils.FileExists(filepath.Join(path, pack.Vendor+"."+pack.Name+utils.PdscExtension)) {
		status = " (installed)"
	}
	if !lookup.loaded {
		lookup.loaded = true
		index := xml.NewPidxXML(filepath.Join(lookup.root, ".Local", "local_repository.pidx"), false)
		if _, err := os.Stat(index.GetFileName()); err == nil {
			lookup.err = index.Read()
			if lookup.err == nil {
				lookup.tags = index.ListPdscTags()
			}
		} else if !os.IsNotExist(err) {
			lookup.err = err
		}
	}
	if lookup.err != nil {
		return "", lookup.err
	}
	for _, tag := range lookup.tags {
		if tag.Vendor != pack.Vendor || tag.Name != pack.Name {
			continue
		}
		path, err := utils.FileURLToPath(strings.TrimRight(tag.URL, "/") + "/" + tag.VName() + utils.PdscExtension)
		if err != nil {
			return "", err
		}
		pdsc := xml.NewPdscXML(path)
		if err := pdsc.Read(); err != nil {
			return "", err
		}
		if utils.SemverCompare(pdsc.LatestVersion(), pack.Version) == 0 {
			status += fmt.Sprintf(" (installed via %s)", path)
		}
	}
	return status, nil
}
