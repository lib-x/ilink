package ilink

import "strconv"

func itoa(n int) string { return strconv.Itoa(n) }

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func parseInt64(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
