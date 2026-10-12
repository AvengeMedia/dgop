//go:build darwin

package gops

func getPssDirty(_ int32) (uint64, error) {
	return 0, nil
}

type procStat struct {
	Status  string
	Nice    int
	Threads int
}

func readProcStat(_ int32) (procStat, bool) {
	return procStat{}, false
}
