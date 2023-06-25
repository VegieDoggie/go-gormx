package gormx

import (
	"fmt"
	"gorm.io/gorm"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unsafe"
)

const SumFieldsKey = "#sum"

var isInjectionReg = regexp.MustCompile("[^a-zA-Z_]+")

type BaseCondition struct {
	Page     *int    `json:"page" form:"page"`         // zh: 页码
	Pagesize *int    `json:"pagesize" form:"pagesize"` // zh: 每页大小
	OrderKey *string `json:"orderKey" form:"orderKey"` // zh: 排序字段名
	Desc     *bool   `json:"desc" form:"desc"`         // zh: 是否降序
}

type GeneralResult struct {
	Total int64                    `json:"total"`
	List  []map[string]interface{} `json:"list"`
	Sum   map[string]interface{}   `json:"sum"`
}

type WrapDB struct {
	Db               *gorm.DB
	maxPagesize      int
	allowEmptyString int
	page             *int
	pagesize         *int
	orderKey         *string
	desc             *bool
}

func NewWrapDB(db *gorm.DB, maxPagesize, allowEmptyString int) *WrapDB {
	return &WrapDB{Db: db, maxPagesize: maxPagesize, allowEmptyString: allowEmptyString}
}

func (w *WrapDB) QueryWithMap(search map[string]interface{}) (gr GeneralResult, err error) {
	gr = GeneralResult{List: make([]map[string]interface{}, 0), Sum: make(map[string]interface{}), Total: 0}
	for key, val := range search {
		w.doWhere(key, val)
	}
	if err = w.Db.Count(&gr.Total).Error; err != nil {
		return
	}
	if gr.Total > 0 {
		w.tryOrder().tryPage()
		if err = w.Db.Scan(&gr.List).Error; err != nil {
			return
		}
		// [Optional] Underscore field name to UpperCamelCase
		if len(gr.List) > 0 {
			list := make([]map[string]interface{}, 0)
			for _, m := range gr.List {
				nm := make(map[string]interface{})
				for k, v := range m {
					nm[underscoreToUpperCamelCase(k)] = v
				}
				list = append(list, nm)
			}
			gr.List = list
		}
		// page-1 can do sum only
		if w.page != nil && *w.page == 1 {
			var sumFields []string
			if _sumFields := search[SumFieldsKey].([]string); _sumFields != nil {
				for _, sumField := range _sumFields {
					if isInjectionReg.MatchString(sumField) {
						err = fmt.Errorf("Ilegal sumField: %s ", sumField)
						return
					}
					sumFields = append(sumFields, camelCaseToUnderscore(sumField))
				}
			}
			if len(sumFields) > 0 {
				var sb strings.Builder
				for _, field := range sumFields {
					sb.Write([]byte(fmt.Sprintf("sum(`%s`) as `%s`, ", field, field)))
				}
				if sb.Len() > 16 {
					if err = w.Db.Select(sb.String()[:sb.Len()-2]).Scan(gr.Sum).Error; err != nil {
						return
					}
				}
			}
		}
	}
	return
}

func (w *WrapDB) QueryWithStruct(search interface{}, list interface{}, sum interface{}, total *int64) (err error) {
	if search != nil {
		w.doDeepWhere("", reflect.ValueOf(search))
	}
	if total != nil {
		if err = w.Db.Count(total).Error; err != nil {
			return
		}
	}
	if total == nil || *total > 0 {
		w.tryOrder().tryPage()
		if list != nil {
			if err = w.Db.Scan(list).Error; err != nil {
				return
			}
		}
		// page-1 can do sum only
		if w.page != nil && *w.page == 1 && sum != nil {
			sv := reflect.ValueOf(sum)
			sk := sv.Kind()
			switch sk {
			case reflect.Pointer, reflect.UnsafePointer:
				sv = reflect.ValueOf(sv.Elem().Interface())
				sk = sv.Kind()
				if sk == reflect.Struct {
					if t, n := sv.Type(), sv.NumField(); n > 0 {
						var sb strings.Builder
						for i := 0; i < n; i++ {
							ns := camelCaseToUnderscore(t.Field(i).Name)
							sb.Write([]byte(fmt.Sprintf("sum(`%s`) as `%s`, ", ns, ns)))
						}
						if sb.Len() > 0 {
							w.Db.Select(sb.String()[:sb.Len()-2]).Scan(sum)
						}
					}
				}
			default:
				return fmt.Errorf("[SUM] Unknow: %s , only pointer allowed", sum)
			}
		}
	} else {
		if list != nil {
			// set empty array
			s := (*reflect.SliceHeader)(reflect.ValueOf(list).UnsafePointer())
			if s.Data == 0 {
				e := make([]interface{}, 0)
				s.Data = (uintptr)(unsafe.Pointer(&e))
			}
		}
	}
	return
}

func camelCaseToUnderscore(s string) string {
	var output []rune
	output = append(output, unicode.ToLower(rune(s[0])))
	for i := 1; i < len(s); i++ {
		if unicode.IsUpper(rune(s[i])) {
			output = append(output, '_')
		}
		output = append(output, unicode.ToLower(rune(s[i])))
	}
	return string(output)
}

func underscoreToUpperCamelCase(s string) string {
	var output []rune
	for i, f := 0, false; i < len(s); i++ {
		if s[i] == '_' {
			f = true
			continue
		}
		if f {
			f = false
			output = append(output, unicode.ToUpper(rune(s[i])))
		} else {
			output = append(output, rune(s[i]))
		}
	}
	return string(output)
}

func (w *WrapDB) tryOrder() *WrapDB {
	if w.orderKey != nil {
		if orderKey := *w.orderKey; orderKey != "" && !isInjectionReg.MatchString(orderKey) {
			if w.desc != nil && *w.desc {
				orderKey += " desc"
			}
			w.Db.Order(orderKey)
		}
	}
	return w
}

func (w *WrapDB) tryPage() *WrapDB {
	if w.pagesize == nil || *w.pagesize > w.maxPagesize {
		w.pagesize = &w.maxPagesize
	}
	if *w.pagesize > 0 {
		w.Db.Limit(*w.pagesize)
	}
	if w.page != nil {
		w.Db.Offset(*w.pagesize * (*w.page - 1))
	}
	return w
}

func (w *WrapDB) doWhere(key string, val interface{}) *WrapDB {
	if len(key) == 0 || strings.HasPrefix(key, "#") {
		return w
	}
	if w.allowEmptyString < 1 {
		if ref := reflect.ValueOf(val); ref.Kind() == reflect.String && ref.String() == "" {
			return w
		}
	}
	db := w.Db
	key = camelCaseToUnderscore(key)
	switch key {
	case "page":
		var page int
		ref := reflect.ValueOf(val)
		if ref.CanFloat() {
			page = int(ref.Float())
		} else if ref.CanInt() {
			page = int(ref.Int())
		} else {
			page = int(ref.Uint())
		}
		w.page = &page
	case "pagesize", "page_size":
		var pagesize int
		ref := reflect.ValueOf(val)
		if ref.CanFloat() {
			pagesize = int(ref.Float())
		} else if ref.CanInt() {
			pagesize = int(ref.Int())
		} else {
			pagesize = int(ref.Uint())
		}
		w.pagesize = &pagesize
	case "orderkey", "order_key":
		value := camelCaseToUnderscore(val.(string))
		if strings.HasPrefix(value, "desc_") {
			n, b := value[5:], true
			w.orderKey, w.desc = &n, &b
		} else {
			var n string
			if strings.HasPrefix(value, "asc_") {
				n = value[4:]
			} else {
				n = value
			}
			w.orderKey = &n
		}
	default:
		index := strings.Index(key, "_")
		if index != -1 {
			prefix := key[:index]
			index++
			switch prefix {
			case "neq":
				db.Where(fmt.Sprintf("`%s` <> ?", key[index:]), val)
			case "gt":
				db.Where(fmt.Sprintf("`%s` > ?", key[index:]), val)
			case "gte":
				db.Where(fmt.Sprintf("`%s` >= ?", key[index:]), val)
			case "lt":
				db.Where(fmt.Sprintf("`%s` < ?", key[index:]), val)
			case "lte":
				db.Where(fmt.Sprintf("`%s` <= ?", key[index:]), val)
			case "in":
				db.Where(fmt.Sprintf("`%s` in ?", key[index:]), val)
			case "nin":
				db.Where(fmt.Sprintf("`%s` not in ?", key[index:]), val)
			case "like":
				db.Where(fmt.Sprintf("`%s` like ?", key[index:]), val)
			case "nlike":
				db.Where(fmt.Sprintf("`%s` not like ?", key[index:]), val)
			case "eq":
				db.Where(fmt.Sprintf("`%s` = ?", key[index:]), val)
			default:
				db.Where(fmt.Sprintf("`%s` = ?", key), val)
			}
		} else {
			db.Where(fmt.Sprintf("`%s` = ?", key), val)
		}
	}
	return w
}

func (w *WrapDB) doDeepWhere(k string, v reflect.Value) *WrapDB {
	kind := v.Kind()
	switch kind {
	case reflect.Pointer, reflect.UnsafePointer:
		if !v.IsNil() {
			w.doDeepWhere(k, v.Elem())
		}
	case reflect.Struct:
		t, n := v.Type(), v.NumField()
		for i := 0; i < n; i++ {
			ki, vi := t.Field(i).Name, v.Field(i)
			kind = vi.Kind()
			switch kind {
			case reflect.Pointer, reflect.UnsafePointer:
				w.doDeepWhere(ki, vi.Elem())
			case reflect.Struct, reflect.Map:
				w.doDeepWhere("", vi)
			default:
				w.doWhere(ki, vi.Interface())
			}
		}
	case reflect.Map:
		if keys := v.MapKeys(); len(keys) > 0 && keys[0].Kind() == reflect.String {
			for _, key := range keys {
				w.doWhere(key.String(), v.MapIndex(key).Interface())
			}
		}
	default:
		w.doWhere(k, v.Interface())
	}
	return w
}
