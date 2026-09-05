package dymsg

import "strconv"

type pathSeg struct {
	field  string
	idx    int
	hasIdx bool
}

// parsePathBuf parses a field path expression into segments, appending to the
// caller-provided buffer (which should have spare capacity) so that typical
// paths need no heap allocation. An empty path yields no segments (the message
// itself).
func parsePathBuf(path string, buf []pathSeg) ([]pathSeg, error) {
	if path == "" {
		return buf[:0], nil
	}
	segs := buf[:0]
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
