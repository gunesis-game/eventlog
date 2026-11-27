package fieldschema

// TypeKey identifies a concrete Go type by reflected package path and name.
type TypeKey struct {
	PackagePath string
	Name        string
}

func (k TypeKey) String() string {
	return k.PackagePath + "." + k.Name
}
