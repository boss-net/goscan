package urlutil

import (
	"strings"
)

// AutoMergeRelPaths merges two relative paths including parameters and returns final string
func AutoMergeRelPaths(path1 string, path2 string) (string, error) {
	u1, err1 := Parse(path1)
	if err1 != nil {
		return "", err1
	}
	u2, err2 := Parse(path2)
	if err2 != nil {
		return "", err2
	}
	u1.MergePath(u2.Path, false)
	u1.Params.Merge(u2.Params)
	return u1.GetRelativePath(), nil
}

// mergePaths merges two relative paths
func mergePaths(elem1 string, elem2 string) string {
	// if both have slash remove one
	if strings.HasSuffix(elem1, "/") && strings.HasPrefix(elem2, "/") {
		elem2 = strings.TrimLeft(elem2, "/")
	}

	if elem1 == "" {
		return elem2
	} else if elem2 == "" {
		return elem1
	}

	// if both paths donot have a slash add it to beginning of second
	if !strings.HasSuffix(elem1, "/") && !strings.HasPrefix(elem2, "/") {
		elem2 = "/" + elem2
	}

	// Do not normalize but combibe paths same as path.join
	/*
		Merge Examples (Same as path.Join)
		/blog   /admin => /blog/admin
		/blog/wp /wp-content  => /blog/wp/wp-content
		/blog/admin /blog/admin/profile => /blog/admin/profile
		/blog/admin /blog => /blog/admin/blog
		/blog /blog/ => /blog/
	*/

	if elem1 == elem2 {
		return elem1
	} else if len(elem1) > len(elem2) && strings.HasSuffix(elem1, elem2) {
		return elem1
	} else if len(elem1) < len(elem2) && strings.HasPrefix(elem2, elem1) {
		return elem2
	} else {
		return elem1 + elem2
	}
}

// ShouldEscapes returns true if given payload contains any characters
// that are not accepted by url or must be escaped
// this does not include seperators
func shouldEscape(ss string) bool {
	rmap := getrunemap(RFCEscapeCharSet)
	for _, v := range ss {
		switch {
		case v == '/':
			continue
		case v > rune(127):
			return true
		default:
			if _, ok := rmap[v]; ok {
				return true
			}
		}
	}
	return false
}
