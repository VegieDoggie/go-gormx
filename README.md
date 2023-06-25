# Go-Gormx

基于 gorm 扩展的通用查询接口，主要用于开发后台应用的查询接口

## 预览

- 提供基于Map的组合条件查询接口，支持分页，排序，汇总和计数等
- 提供基于Struct的组合条件查询接口，支持分页，排序，汇总和计数等

## 快速开始

```go
package example

import (
	"encoding/json"
	"github.com/VegetableDoggie/go-gormx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"testing"
)

// TestMapUser Map 接口查询示例
func TestMapUser(t *testing.T) {
	db, err := gorm.Open(mysql.Open("root:root@tcp(127.0.0.1:3306)/gormx_test?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	if err != nil {
		panic("failed to connect database")
	}
	userDb := gormx.NewWrapDB(db.Table("user"), 10, false)

	search := make(map[string]interface{})
	search["id"] = append(make([]uint, 0), 3)
	search["ninId"] = append(make([]uint, 0), 1, 2)
	search["likeName"] = "_oo"
	search["page"] = 1
	search["pagesize"] = 2
	search["#sum"] = []string{"level"}
	search["orderKey"] = "ascId"

	gr, err := userDb.QueryWithMap(search)
	if err != nil {
		log.Println(err)
	}
	marshal, _ := json.Marshal(gr)
	log.Println(string(marshal))
	// SELECT sum(`level`) as `level` FROM `user` WHERE `id` = (3) AND `id` not in (1,2) AND `name` like '_oo' ORDER BY id LIMIT 2
	// {"total":1,"list":[{"createdAt":1682797017126,"id":3,"level":3,"name":"Hoo","status":3,"updatedAt":1682797017126}],"sum":{"level":"3"}}
}

// TestStructUser Struct 接口查询示例
func TestStructUser(t *testing.T) {
	db, err := gorm.Open(mysql.Open("root:root@tcp(127.0.0.1:3306)/gormx_test?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	if err != nil {
		panic("failed to connect database")
	}
	userDb := gormx.NewWrapDB(db.Table("user"), 10, false)

	type User struct {
		Id        uint   `json:"id"`
		Name      string `json:"name"`
		Level     uint   `json:"level"`
		Status    uint   `json:"status"`
		CreatedAt uint   `json:"createdAt"`
		UpdatedAt uint   `json:"updatedAt"`
	}
	search := &struct {
		Id       uint
		NinId    []uint
		LikeName string
		Page     int
		Pagesize int
		OrderKey string
	}{
		Id:       3,
		NinId:    []uint{1, 2},
		LikeName: "_oo",
		Page:     1,
		Pagesize: 2,
	}
	sum := new(struct {
		Level uint
	})
	list := new([]User)
	total := new(int64)
	err = userDb.QueryWithStruct(search, list, sum, total)
	if err != nil {
		log.Println(err)
	}
	log.Println(list, sum, *total)
	// SELECT sum(`level`) as `level` FROM `user` WHERE `id` = 3 AND `id` not in (1,2) AND `name` like '_oo' LIMIT 2
	// &[{3 Hoo 3 3 1682797017126 1682797017126}] &{3} 1
}
```

## 说明
- Map
    - 优点: 不用再写DAO层查询代码，没有字段类型和数量的限制，前后端唯一的交接工作便是表结构(比如Swagger)。
    - 用途: 尤其适合后台管理系统的通用查询接口
- Struct
    - 优点: 不用再写DAO层查询代码，支持严格限制字段的类型和数量(不同的接口定义不同的结构体)
    - 用途: 后端接口(无论什么系统)