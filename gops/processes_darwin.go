//go:build darwin

package gops

func getPssDirty(_ int32) (uint64, error) {
	return 0, nil
}
