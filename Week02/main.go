package main

import (
	"database/sql"
	"errors"
	xerrors "github.com/pkg/errors"
	"log"
)

//我们在数据库操作的时候，比如 dao 层中当遇到一个 sql.ErrNoRows 的时候，
//是否应该 Wrap 这个 error，抛给上层。为什么，应该怎么做请写出代码？

var (
	DataRowNotFount = errors.New("rows: data row not found")
)

// 数据库对象
type DataBaseObj struct {
	Value        interface{}
	Error        error
	RowsAffected int64
}

// 数据库结构
type People struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
	//....
}

// 执行sql的方法
func (item *DataBaseObj) ExecuteSQL() interface{} {
	// 没有查到数据 返回nil err放到公共对象中
	item.Error = sql.ErrNoRows
	item.RowsAffected = 0
	return nil
}

// 假装是 orm Select方法
func (item *DataBaseObj) Select(query interface{}) error {
	// 假装操作数据库
	log.Print("query:", query)
	item.ExecuteSQL()
	if err := item.Error; err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = DataRowNotFount
		}
		return xerrors.Wrap(err, "Select func")
	}

	return nil
}

// dao层数据操作层
func (item *People) PeopleSelectList() ([]*People, error) {
	list := make([]*People, 0)
	db := &DataBaseObj{}
	db.Value = list
	err := db.Select(item.Class)
	return list, err
}

func main() {
	people := &People{Class: "六班"}
	list, err := people.PeopleSelectList()
	if err != nil {
		log.Printf("People list data failed: %+v\n", err)
	}
	log.Printf("People list: %+v\n", list)
}
