package tags

import "reflect"

// AssignReflectValue attempts to set target to value, converting if needed.
func AssignReflectValue(target, value reflect.Value) bool {
	if !target.CanSet() || !value.IsValid() {
		return false
	}
	if value.Type().AssignableTo(target.Type()) {
		target.Set(value)
		return true
	}
	if value.Type().ConvertibleTo(target.Type()) {
		target.Set(value.Convert(target.Type()))
		return true
	}
	return false
}

// SetMapValue sets key→value on target map, coercing value to the map's
// element type. Returns false if assignment fails.
func SetMapValue(target, key, value reflect.Value) bool {
	elem := reflect.New(target.Type().Elem()).Elem()
	if !AssignReflectValue(elem, value) {
		return false
	}
	target.SetMapIndex(key, elem)
	return true
}
