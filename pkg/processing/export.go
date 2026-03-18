package processing

// CopyTree copies a directory tree from src to dst.
func CopyTree(src, dst string) error {
	return copyTree(src, dst)
}
