//go:build !linux && !darwin

package handlers

func diskSpace(string) (free, total uint64, ok bool) { return 0, 0, false }
