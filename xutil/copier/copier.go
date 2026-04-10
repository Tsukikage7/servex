package copier

import (
	"reflect"
)

// Copy 从 src 创建 Dst 类型的新对象，按字段名匹配复制.
func Copy[Dst, Src any](src *Src) (*Dst, error) {
	if src == nil {
		return nil, ErrNilSource
	}
	dst := new(Dst)
	if err := copyStruct(reflect.ValueOf(src).Elem(), reflect.ValueOf(dst).Elem(), defaultCopyOptions()); err != nil {
		return nil, err
	}
	return dst, nil
}

// CopyTo 将 src 的字段值复制到已有的 dst 对象.
func CopyTo[Dst, Src any](src *Src, dst *Dst) error {
	if src == nil {
		return ErrNilSource
	}
	if dst == nil {
		return ErrNilDestination
	}
	return copyStruct(reflect.ValueOf(src).Elem(), reflect.ValueOf(dst).Elem(), defaultCopyOptions())
}

// CopyWithOptions 从 src 创建 Dst 类型的新对象，按字段名匹配复制，支持自定义选项.
func CopyWithOptions[Dst, Src any](src *Src, opts ...CopyOption) (*Dst, error) {
	if src == nil {
		return nil, ErrNilSource
	}
	dst := new(Dst)
	if err := copyStruct(reflect.ValueOf(src).Elem(), reflect.ValueOf(dst).Elem(), applyCopyOptions(opts)); err != nil {
		return nil, err
	}
	return dst, nil
}

// CopyToWithOptions 将 src 的字段值复制到已有的 dst 对象，支持自定义选项.
func CopyToWithOptions[Dst, Src any](src *Src, dst *Dst, opts ...CopyOption) error {
	if src == nil {
		return ErrNilSource
	}
	if dst == nil {
		return ErrNilDestination
	}
	return copyStruct(reflect.ValueOf(src).Elem(), reflect.ValueOf(dst).Elem(), applyCopyOptions(opts))
}

func copyStruct(srcVal, dstVal reflect.Value, opts *copyOptions) error {
	srcType := srcVal.Type()
	dstType := dstVal.Type()

	if srcType.Kind() != reflect.Struct {
		return ErrNotStruct
	}
	if dstType.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	dstFields := make(map[string]int, dstType.NumField())
	for i := range dstType.NumField() {
		f := dstType.Field(i)
		if f.IsExported() {
			dstFields[f.Name] = i
		}
	}

	for i := range srcType.NumField() {
		srcField := srcType.Field(i)
		if !srcField.IsExported() {
			continue
		}
		if opts.ignoreFields[srcField.Name] {
			continue
		}

		dstFieldName := srcField.Name
		if mapped, ok := opts.fieldMapping[srcField.Name]; ok {
			dstFieldName = mapped
		}

		dstIdx, ok := dstFields[dstFieldName]
		if !ok {
			continue
		}

		srcFieldVal := srcVal.Field(i)
		dstFieldVal := dstVal.Field(dstIdx)

		if !dstFieldVal.CanSet() {
			continue
		}

		if err := copyField(srcFieldVal, dstFieldVal, opts); err != nil {
			return err
		}
	}

	return nil
}

func copyField(src, dst reflect.Value, opts *copyOptions) error {
	srcType := src.Type()
	dstType := dst.Type()

	if srcType == dstType {
		// 切片和 map 需要深拷贝，避免源和目标共享底层数据
		switch srcType.Kind() {
		case reflect.Slice:
			if src.IsNil() {
				dst.Set(src)
			} else {
				cp := reflect.MakeSlice(srcType, src.Len(), src.Len())
				reflect.Copy(cp, src)
				dst.Set(cp)
			}
			return nil
		case reflect.Map:
			if src.IsNil() {
				dst.Set(src)
			} else {
				cp := reflect.MakeMapWithSize(srcType, src.Len())
				iter := src.MapRange()
				for iter.Next() {
					cp.SetMapIndex(iter.Key(), iter.Value())
				}
				dst.Set(cp)
			}
			return nil
		default:
			dst.Set(src)
			return nil
		}
	}

	if srcType.AssignableTo(dstType) {
		dst.Set(src)
		return nil
	}

	if srcType.ConvertibleTo(dstType) {
		dst.Set(src.Convert(dstType))
		return nil
	}

	if srcType.Kind() == reflect.Struct && dstType.Kind() == reflect.Struct {
		return copyStruct(src, dst, opts)
	}

	if srcType.Kind() == reflect.Ptr && dstType.Kind() == reflect.Ptr {
		if src.IsNil() {
			return nil
		}
		srcElem := src.Elem()
		dstElem := reflect.New(dstType.Elem())
		if srcElem.Type().Kind() == reflect.Struct && dstType.Elem().Kind() == reflect.Struct {
			if err := copyStruct(srcElem, dstElem.Elem(), opts); err != nil {
				return err
			}
			dst.Set(dstElem)
		}
		return nil
	}

	if srcType.Kind() == reflect.Ptr && dstType.Kind() != reflect.Ptr {
		if !src.IsNil() {
			return copyField(src.Elem(), dst, opts)
		}
		return nil
	}

	if srcType.Kind() != reflect.Ptr && dstType.Kind() == reflect.Ptr {
		if dstType.Elem() == srcType {
			ptr := reflect.New(srcType)
			ptr.Elem().Set(src)
			dst.Set(ptr)
		}
		return nil
	}

	return nil
}
