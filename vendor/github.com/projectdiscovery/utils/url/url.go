package urlutil

import (
	"bytes"
	"log"
	"net/url"
	"strings"

	errorutil "github.com/projectdiscovery/utils/errors"
	stringsutil "github.com/projectdiscovery/utils/strings"
)

// disables autocorrect related to parsing
// Ex: if input is admin url.Parse considers admin as host which is not a valid domain name
var DisableAutoCorrect bool

// URL a wrapper around net/url.URL
type URL struct {
	*url.URL

	Original   string // original or given url(without params if any)
	Unsafe     bool   // If request is unsafe (skip validation)
	IsRelative bool   // If URL is relative
	Params     Params // Query Parameters
	// should call Update() method when directly updating wrapped url.URL or parameters
}

// mergepath merges given relative path
func (u *URL) MergePath(newrelpath string, unsafe bool) error {
	ux, err := ParseURL(newrelpath, unsafe)
	if err != nil {
		return err
	}
	u.Params.Merge(ux.Params)
	u.Path = mergePaths(u.Path, ux.Path)
	if ux.Fragment != "" {
		u.Fragment = ux.Fragment
	}
	return nil
}

// Updates internal wrapped url.URL with any changes done to Query Parameters
func (u *URL) Update() {
	// This is a hot patch for url.URL
	// parameters are serialized when parsed with `url.Parse()` to avoid this
	// url should be parsed without parameters and then assigned with url.RawQuery to force unserialized parameters
	u.RawQuery = u.Params.Encode()
}

// Query returns Query Params
func (u *URL) Query() Params {
	return u.Params
}

// Clone
func (u *URL) Clone() *URL {
	var userinfo *url.Userinfo
	if u.User != nil {
		// userinfo is immutable so this is the only way
		tempurl := "https://" + u.User.String() + "@" + "scanme.sh/"
		turl, _ := url.Parse(tempurl)
		if turl != nil {
			userinfo = turl.User
		}
	}
	ux := &url.URL{
		Scheme:   u.Scheme,
		Opaque:   u.Opaque,
		User:     userinfo,
		Host:     u.Host,
		Path:     u.Path,
		RawPath:  u.RawPath,
		RawQuery: u.RawQuery,
		Fragment: u.Fragment,
		// OmitHost:    u.OmitHost, // only supported in 1.19
		ForceQuery:  u.ForceQuery,
		RawFragment: u.RawFragment,
	}
	params := make(Params)
	if u.Params != nil {
		for k, v := range u.Params {
			params[k] = v
		}
	}
	return &URL{
		URL:        ux,
		Params:     params,
		Original:   u.Original,
		Unsafe:     u.Unsafe,
		IsRelative: u.IsRelative,
	}
}

// String
func (u *URL) String() string {
	var buff bytes.Buffer
	if u.Scheme != "" {
		buff.WriteString(u.Scheme + "://")
	}
	if u.User != nil {
		buff.WriteString(u.User.String())
		buff.WriteRune('@')
	}
	buff.WriteString(u.Host)
	buff.WriteString(u.GetRelativePath())
	return buff.String()
}

// GetRelativePath ex: /some/path?param=true#fragment
func (u *URL) GetRelativePath() string {
	var buff bytes.Buffer
	if u.Path != "" {
		if !strings.HasPrefix(u.Path, "/") {
			buff.WriteRune('/')
		}
		buff.WriteString(u.Path)
	}
	if len(u.Params) > 0 {
		buff.WriteRune('?')
		buff.WriteString(u.Params.Encode())
	}
	if u.Fragment != "" {
		buff.WriteRune('#')
		buff.WriteString(u.Fragment)
	}
	return buff.String()
}

// Updates port
func (u *URL) UpdatePort(newport string) {
	if u.URL.Port() != "" {
		u.Host = strings.Replace(u.Host, u.Port(), newport, 1)
	}
	u.Host += ":" + newport
}

// parseRelativePath parses relative path from Original Path without relying on
// net/url.URL
func (u *URL) parseRelativePath() {
	// url.Parse discards %0a or any percent encoded characters from path
	// to avoid this if given url is not relative but has encoded chars
	// parse the path manually regardless if it is unsafe
	// ex: /%20test%0a =?

	// percent encoding in path
	if u.Host == "" || len(u.Host) < 4 {
		if shouldEscape(u.Original) {
			// we assume it as relative url
			u.IsRelative = true
			u.Path = u.Original
		}
		return
	}
	expectedPath := strings.SplitN(u.Original, u.Host, 2)
	if len(expectedPath) != 2 {
		// something went wrong
		log.Printf("[urlutil] failed to extract path from input url.falling back to defaults..")
		return
	}
	u.Path = expectedPath[1]
}

// fetchParams retrieves query parameters from URL
func (u *URL) fetchParams() {
	if u.Params == nil {
		u.Params = make(Params)
	}
	// parse fragments if any
	if i := strings.IndexRune(u.Original, '#'); i != -1 {
		// assuming ?param=value#highlight
		u.Fragment = u.Original[i+1:]
		u.Original = u.Original[:i]
	}
	if index := strings.IndexRune(u.Original, '?'); index == -1 {
		return
	} else {
		encodedParams := u.Original[index+1:]
		u.Params.Decode(encodedParams)
		u.Original = u.Original[:index]
	}
	u.Update()
}

// Parse and return URL
func ParseURL(inputURL string, unsafe bool) (*URL, error) {
	u := &URL{
		URL:      &url.URL{},
		Original: inputURL,
		Unsafe:   unsafe,
	}
	u.fetchParams()
	// filter out fragments and parameters only then parse path
	inputURL = u.Original
	if inputURL == "" {
		return nil, errorutil.NewWithTag("urlutil", "failed to parse url got empty input")
	}

	if strings.HasPrefix(inputURL, "/") && !strings.HasPrefix(inputURL, "//") {
		// this is definitely a relative path
		u.IsRelative = true
		u.Path = u.Original
		return u, nil
	}
	// Try to parse host related input
	if stringsutil.HasPrefixAny(inputURL, "http", "https", "//") || strings.Contains(inputURL, "://") {
		u.IsRelative = false
		urlparse, parseErr := url.Parse(inputURL)
		if parseErr != nil {
			return nil, errorutil.NewWithErr(parseErr).Msgf("failed to parse url")
		}
		copy(u.URL, urlparse)
	} else {
		// if no prefix try to parse it with https
		// if failed we consider it as a relative path and not a full url
		urlparse, parseErr := url.Parse("https://" + inputURL)
		if parseErr != nil {
			// most likely a relativeurl
			u.IsRelative = true
			// TODO: investigate if prefix / should be added
		} else {
			urlparse.Scheme = "" // remove newly added scheme
			copy(u.URL, urlparse)
		}
	}

	// try parsing path
	if u.IsRelative {
		urlparse, parseErr := url.Parse(inputURL)
		if parseErr != nil {
			if !unsafe {
				// should return error if not unsafe url
				return nil, errorutil.NewWithErr(parseErr).WithTag("urlutil").Msgf("failed to parse input url")
			} else {
				// if unsafe do not rely on net/url.Parse
				u.Path = inputURL
			}
		}
		if urlparse != nil {
			copy(u.URL, urlparse)
		}
	} else {
		// if parsing is successful validate and autocorrect
		//ex: when inputURL is admin url.parse considers admin as Host with parsed with https://
		// i.e https://admin which is not valid/accepted domain
		//TODO: Properly Validate using regex
		if u.Host == "" {
			// this is unexpected case return err
			return nil, errorutil.NewWithTag("urlutil", "failed to parse url %v got empty host", inputURL)
		}
		if !strings.Contains(u.Host, ".") && !strings.Contains(u.Host, ":") {
			// this does not look like a valid domain , ipv4 or ipv6
			// consider it as relative
			if !DisableAutoCorrect {
				u.IsRelative = true
				u.Path = inputURL
				u.Host = ""
			}
		}
	}
	if !u.IsRelative && u.Host == "" {
		return nil, errorutil.NewWithTag("urlutil", "failed to parse url `%v`", inputURL).Msgf("got empty host when url is not relative")
	}
	// edgecase where path contains percent encoded chars ex: /%20test%0a
	u.parseRelativePath()
	return u, nil
}

// ParseURL
func Parse(inputURL string) (*URL, error) {
	return ParseURL(inputURL, false)
}

// copy parsed data from src to dst this does not include fragment or params
func copy(dst *url.URL, src *url.URL) {
	dst.Host = src.Host
	// dst.OmitHost = src.OmitHost // only supported in 1.19
	dst.Opaque = src.Opaque
	dst.Path = src.Path
	dst.RawPath = src.RawPath
	dst.Scheme = src.Scheme
	dst.User = src.User
}
