package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/netguard"
	"github.com/ebogdum/keramos/v3/internal/repo"
	"github.com/spf13/cobra"
)

func newSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for keramos packages",
		Long:  "Search configured repositories or Artifact Hub for keramos packages.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newSearchRepoCommand())
	cmd.AddCommand(newSearchHubCommand())

	return cmd
}

func newSearchRepoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "repo <keyword>",
		Short: "Search configured repositories for packages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyword := strings.ToLower(args[0])

			rf, err := repo.LoadRepoFile()
			if nil != err {
				return err
			}

			if 0 == len(rf.Repositories) {
				fmt.Fprintln(cmd.OutOrStdout(), "No repositories configured. Use 'keramos repo add' first.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s %s\n", "NAME", "VERSION", "APP VERSION", "DESCRIPTION")

			found := false
			for _, r := range rf.Repositories {
				idx, fetchErr := repo.FetchIndex(r.URL)
				if nil != fetchErr {
					continue
				}
				for name, entries := range idx.Entries {
					// Match the keyword against the chart name OR any entry's
					// description/keywords, not just the name.
					if !matchesKeywordBroad(name, entries, keyword) {
						continue
					}
					if 0 == len(entries) {
						continue
					}
					latest := entries[0]
					fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s %s\n",
						r.Name+"/"+name, latest.Version, latest.AppVersion, latest.Description)
					found = true
				}
			}

			if !found {
				fmt.Fprintf(cmd.OutOrStdout(), "No results found for %q\n", keyword)
			}

			return nil
		},
	}
}

func newSearchHubCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hub <keyword>",
		Short: "Search Artifact Hub for packages",
		Long:  "Search Artifact Hub (or another compatible endpoint) for package archives.",
		Args:  cobra.ExactArgs(1),
	}
	var (
		endpoint    string
		kind        int
		limit       int
		maxColWidth int
		regexp_     bool
	)
	cmd.Flags().StringVar(&endpoint, "endpoint", "https://artifacthub.io", "Artifact Hub-compatible endpoint")
	cmd.Flags().IntVar(&kind, "kind", 0, "package kind code per the index endpoint (default 0; e.g. 1=Falco, 14=OCI)")
	cmd.Flags().IntVar(&limit, "max-results", 20, "maximum results to display")
	cmd.Flags().IntVar(&maxColWidth, "max-col-width", 50, "maximum column width for output")
	cmd.Flags().BoolVar(&regexp_, "regexp", false, "treat the keyword as a regular expression")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		results, err := searchArtifactHubAt(endpoint, args[0], kind, limit)
		if nil != err {
			return err
		}
		// --regexp: post-filter the hub's fuzzy matches by an actual regular
		// expression over the package name and description.
		if regexp_ {
			re, reErr := regexp.Compile(args[0])
			if nil != reErr {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "invalid --regexp pattern: %v", reErr)
			}
			kept := make([]hubResult, 0, len(results))
			for _, p := range results {
				if re.MatchString(p.URL) || re.MatchString(p.Description) {
					kept = append(kept, p)
				}
			}
			results = kept
		}
		if 0 == len(results) {
			fmt.Fprintf(cmd.OutOrStdout(), "No results found for %q\n", args[0])
			return nil
		}
		w := maxColWidth
		if w < 1 {
			w = 1
		}
		trunc := func(s string) string {
			if len(s) > w {
				return s[:w]
			}
			return s
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-*s %-15s %-15s %-*s %s\n",
			w, "NAME", "VERSION", "APP VERSION", w, "REPO URL", "DESCRIPTION")
		for _, p := range results {
			fmt.Fprintf(cmd.OutOrStdout(), "%-*s %-15s %-15s %-*s %s\n",
				w, trunc(p.URL), p.Version, p.AppVersion, w, trunc(p.RepoURL), p.Description)
		}
		return nil
	}
	return cmd
}

// hubResult is a slim subset of Artifact Hub's package search response.
type hubResult struct {
	URL         string
	Version     string
	AppVersion  string
	Description string
	RepoURL     string
}

// searchArtifactHubAt queries an Artifact Hub-compatible endpoint and returns
// up to `limit` packages of the given kind code (0 selects the index's
// default chart kind). Refuses non-HTTPS endpoints to prevent SSRF and
// plaintext credential leakage.
func searchArtifactHubAt(endpoint, keyword string, kind, limit int) ([]hubResult, error) {
	if !strings.HasPrefix(strings.ToLower(endpoint), "https://") {
		return nil, fmt.Errorf("search hub endpoint must use https:// (got %q)", endpoint)
	}
	url := fmt.Sprintf("%s/api/v1/packages/search?ts_query_web=%s&kind=%d&limit=%d",
		strings.TrimRight(endpoint, "/"), urlQueryEscape(keyword), kind, limit)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if nil != err {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	client := netguard.HTTPClient(netguard.BlockMetadata, "KERAMOS_ALLOW_INTERNAL_FETCH", 15*time.Second)
	resp, err := client.Do(req)
	if nil != err {
		return nil, err
	}
	defer resp.Body.Close()
	if 200 != resp.StatusCode {
		return nil, fmt.Errorf("artifacthub returned HTTP %d", resp.StatusCode)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if nil != readErr {
		return nil, readErr
	}

	var raw struct {
		Packages []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
			AppVersion  string `json:"app_version"`
			Repository  struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"repository"`
		} `json:"packages"`
	}
	if jsonErr := json.Unmarshal(body, &raw); nil != jsonErr {
		// Some artifacthub deployments return a bare array.
		var arr []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
			AppVersion  string `json:"app_version"`
			Repository  struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"repository"`
		}
		if jsonErr2 := json.Unmarshal(body, &arr); nil != jsonErr2 {
			return nil, jsonErr2
		}
		out := make([]hubResult, 0, len(arr))
		for _, p := range arr {
			out = append(out, hubResult{
				URL:         p.Repository.Name + "/" + p.Name,
				Version:     p.Version,
				AppVersion:  p.AppVersion,
				Description: p.Description,
				RepoURL:     p.Repository.URL,
			})
		}
		return out, nil
	}
	out := make([]hubResult, 0, len(raw.Packages))
	for _, p := range raw.Packages {
		out = append(out, hubResult{
			URL:         p.Repository.Name + "/" + p.Name,
			Version:     p.Version,
			AppVersion:  p.AppVersion,
			Description: p.Description,
			RepoURL:     p.Repository.URL,
		})
	}
	return out, nil
}

func urlQueryEscape(s string) string {
	out := make([]byte, 0, len(s)*2)
	for _, c := range []byte(s) {
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', '-' == c, '_' == c, '.' == c, '~' == c:
			out = append(out, c)
		default:
			out = append(out, '%', hexDigit(c>>4), hexDigit(c&0x0f))
		}
	}
	return string(out)
}

func hexDigit(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'a' + v - 10
}

func matchesKeyword(name, keyword string) bool {
	return strings.Contains(strings.ToLower(name), keyword)
}

// matchesKeywordBroad matches the keyword against a name OR any of the
// entries' descriptions. Used by the broader-match path of `keramos search`.
func matchesKeywordBroad(name string, entries []repo.IndexEntry, keyword string) bool {
	if strings.Contains(strings.ToLower(name), keyword) {
		return true
	}
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Description), keyword) {
			return true
		}
	}
	return false
}
