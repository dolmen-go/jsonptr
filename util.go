package jsonptr

// MustValue allows to wrap a call to some jsonptr function which returns a value to transform any error into a panic.
func MustValue(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}
