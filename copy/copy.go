package copy

import (
	"fmt"
	"reflect"
	"time"
)

// AssignStruct 将src中有值的字段赋值到dst中
//
// - 是将相同字段名中src值赋给dst中对应字段
// - 入参必须是结构体对象引用
// - 若结构体中存在切片, 请先初始化至src\dst一致
// - 如果存在内联, 保证内联结构体名称一致
func AssignStruct(src, dst interface{}) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
		}
	}()
	if src == nil || reflect.ValueOf(src).IsNil() ||
		dst == nil || reflect.ValueOf(dst).IsNil() {
		fmt.Println("src or dst is nil")
		return
	}
	assignStructFields(reflect.ValueOf(src).Elem(), reflect.ValueOf(dst).Elem())
}

// assignStructFields 将src中有值的字段赋值到dst中, 递归至成员变量最小类型
func assignStructFields(src, dst reflect.Value) {
	srcType := src.Type()
	for i := 0; i < srcType.NumField(); i++ {
		field := srcType.Field(i)
		fieldName := field.Name

		srcFieldValue := src.FieldByName(fieldName)
		dstFieldValue := dst.FieldByName(fieldName)

		// 如果字段是匿名的（内嵌的），但在 dst 中不存在，则尝试将 src 内嵌字段的子字段拷贝到 dst 中
		if field.Anonymous && !dstFieldValue.IsValid() {
			// 如果 srcFieldValue 是结构体，则直接将其字段拷贝到 dst 中
			if srcFieldValue.Kind() == reflect.Struct {
				assignStructFields(srcFieldValue, dst)
			}
			continue
		}

		// 检查字段是否有效
		if srcFieldValue.IsValid() && dstFieldValue.IsValid() {
			// 检查 srcFieldValue 是否为 nil 指针
			if srcFieldValue.Kind() == reflect.Ptr && srcFieldValue.IsNil() {
				continue
			}
			// 如果字段值为零值，则跳过
			if srcFieldValue.IsZero() {
				continue
			}

			// 对于 time.Time 类型特殊处理
			if field.Type == reflect.TypeOf(time.Time{}) {
				dstFieldValue.Set(srcFieldValue)
				continue
			}

			// 如果字段是结构体，则递归处理
			if srcFieldValue.Kind() == reflect.Struct {
				assignStructFields(srcFieldValue, dstFieldValue)
				continue
			}

			// 如果字段是 slice，则调用相应的处理函数
			if srcFieldValue.Kind() == reflect.Slice {
				assignSliceFields(srcFieldValue, dstFieldValue)
				continue
			}

			// 如果类型匹配，则直接设置
			if srcFieldValue.Kind() == dstFieldValue.Kind() {
				dstFieldValue.Set(srcFieldValue)
			}
		}
	}
}

// assignSliceFields 复制切片
func assignSliceFields(src, dst reflect.Value) {
	elemType := src.Type().Elem()
	// 若元素类型是结构体且源切片元素个数等于目标切片元素个数时, 依次递归复制
	if elemType.Kind() == reflect.Struct && src.Len() == dst.Len() {
		// 依次处理每个元素
		for j := 0; j < src.Len(); j++ {
			assignStructFields(src.Index(j), dst.Index(j))
		}
	} else {
		if src.Kind() == dst.Kind() {
			dst.Set(src)
		}
	}
}

// DeepCopy creates a deep copy of whatever is passed to it and returns the copy
// in an interface{}.  The returned value will need to be asserted to the
// correct type.
func DeepCopy(src interface{}) interface{} {
	if src == nil {
		return nil
	}

	// Make the interface a reflect.Value
	original := reflect.ValueOf(src)

	// Make a copy of the same type as the original.
	cpy := reflect.New(original.Type()).Elem()

	// Recursively copy the original.
	copyRecursive(original, cpy)

	// Return the copy as an interface.
	return cpy.Interface()
}

// Interface for delegating copy process to type
type Interface interface {
	DeepCopy() interface{}
}

// copyRecursive does the actual copying of the interface. It currently has
// limited support for what it can handle. Add as needed.
func copyRecursive(original, cpy reflect.Value) {
	// check for implement deepcopy.Interface
	if original.CanInterface() {
		if copier, ok := original.Interface().(Interface); ok {
			cpy.Set(reflect.ValueOf(copier.DeepCopy()))
			return
		}
	}

	// handle according to original's Kind
	switch original.Kind() {
	case reflect.Ptr:
		// Get the actual value being pointed to.
		originalValue := original.Elem()

		// if  it isn't valid, return.
		if !originalValue.IsValid() {
			return
		}
		cpy.Set(reflect.New(originalValue.Type()))
		copyRecursive(originalValue, cpy.Elem())

	case reflect.Interface:
		// If this is a nil, don't do anything
		if original.IsNil() {
			return
		}
		// Get the value for the interface, not the pointer.
		originalValue := original.Elem()

		// Get the value by calling Elem().
		copyValue := reflect.New(originalValue.Type()).Elem()
		copyRecursive(originalValue, copyValue)
		cpy.Set(copyValue)

	case reflect.Struct:
		t, ok := original.Interface().(time.Time)
		if ok {
			cpy.Set(reflect.ValueOf(t))
			return
		}
		// Go through each field of the struct and copy it.
		for i := 0; i < original.NumField(); i++ {
			// The Type's StructField for a given field is checked to see if StructField.PkgPath
			// is set to determine if the field is exported or not because CanSet() returns false
			// for settable fields.  I'm not sure why.  -mohae
			if original.Type().Field(i).PkgPath != "" {
				continue
			}
			copyRecursive(original.Field(i), cpy.Field(i))
		}

	case reflect.Slice:
		if original.IsNil() {
			return
		}
		// Make a new slice and copy each element.
		cpy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i++ {
			copyRecursive(original.Index(i), cpy.Index(i))
		}

	case reflect.Map:
		if original.IsNil() {
			return
		}
		cpy.Set(reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			copyValue := reflect.New(originalValue.Type()).Elem()
			copyRecursive(originalValue, copyValue)
			copyKey := DeepCopy(key.Interface())
			cpy.SetMapIndex(reflect.ValueOf(copyKey), copyValue)
		}

	default:
		cpy.Set(original)
	}
}
