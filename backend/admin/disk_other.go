//go:build !linux && !darwin

package admin

func diskSpace(string) (free, total uint64, ok bool) { return 0, 0, false }
