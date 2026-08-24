package dymsg

import "strconv"

type pathSeg struct {
	field  string
	idx    int
	hasIdx bool
}

// parsePath parses a field path expression into segments. An empty path yields
// nil segments (the message itself).
func parsePath(path string) ([]pathSeg, error) {
	if path == "" {
		return nil, nil
	}
	var segs []pathSeg
	pos := 0
	n := len(path)
	for pos < n {
		start := pos
		for pos < n && path[pos] != '.' && path[pos] != '[' {
			pos++
		}
		name := path[start:pos]
		if name == "" {
			return nil, ErrFieldNotFound
		}
		seg := pathSeg{field: name}
		if pos < n && path[pos] == '[' {
			pos++
			idxStart := pos
			for pos < n && path[pos] != ']' {
				pos++
			}
			if pos >= n {
				return nil, ErrFieldNotFound
			}
			if err := parseIndexContent(path[idxStart:pos], &seg); err != nil {
				return nil, err
			}
			pos++ // consume ']'
			if pos < n && path[pos] != '.' {
				return nil, ErrFieldNotFound
			}
		}
		segs = append(segs, seg)
		if pos < n {
			// must be '.'
			if path[pos] != '.' {
				return nil, ErrFieldNotFound
			}
			pos++
			if pos >= n {
				return nil, ErrFieldNotFound
			}
		}
	}
	return segs, nil
}

func parseIndexContent(content string, seg *pathSeg) error {
	if content == "" {
		return ErrIndexOutOfRange
	}
	if content[0] == '-' {
		return ErrIndexOutOfRange
	}
	for i := 0; i < len(content); i++ {
		if content[i] < '0' || content[i] > '9' {
			return ErrIndexOutOfRange
		}
	}
	v, err := strconv.Atoi(content)
	if err != nil {
		return ErrIndexOutOfRange
	}
	seg.idx = v
	seg.hasIdx = true
	return nil
}
