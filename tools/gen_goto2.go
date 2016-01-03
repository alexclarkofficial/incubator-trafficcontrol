// Copyright 2015 Comcast Cable Communications Management, LLC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// started from https://github.com/asdf072/struct-create

package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
	"os/exec"
	"strings"
)

var defaults = Configuration{
	DbUser:     os.Args[1],
	DbPassword: os.Args[2],
	DbName:     os.Args[3],
	PkgName:    "api",
	TagLabel:   "db",
}

var config Configuration

type Configuration struct {
	DbUser     string `json:"db_user"`
	DbPassword string `json:"db_password"`
	DbName     string `json:"db_name"`
	// PkgName gives name of the package using the stucts
	PkgName string `json:"pkg_name"`
	// TagLabel produces tags commonly used to match database field names with Go struct members
	TagLabel string `json:"tag_label"`
}

type ColumnSchema struct {
	TableName              string
	ColumnName             string
	IsNullable             string
	DataType               string
	CharacterMaximumLength sql.NullInt64
	NumericPrecision       sql.NullInt64
	NumericScale           sql.NullInt64
	ColumnType             string
	ColumnKey              string
}

func idCol(schemas []ColumnSchema, table string) string {
	for _, cs := range schemas {
		if cs.TableName == table {
			return cs.ColumnName // the first one, it's ordered
		}
	}
	return ""
}

func writeFile(schemas []ColumnSchema, table string) (int, error) {
	file, err := os.Create("./generated/" + table + ".go")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	license := "// Copyright 2015 Comcast Cable Communications Management, LLC\n\n"
	license += "// Licensed under the Apache License, Version 2.0 (the \"License\");\n"
	license += "// you may not use this file except in compliance with the License.\n"
	license += "// You may obtain a copy of the License at\n\n"
	license += "// http://www.apache.org/licenses/LICENSE-2.0\n\n"
	license += "// Unless required by applicable law or agreed to in writing, software\n"
	license += "// distributed under the License is distributed on an \"AS IS\" BASIS,\n"
	license += "// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.\n"
	license += "// See the License for the specific language governing permissions and\n"
	license += "// limitations under the License.\n\n"
	license += "// This file was initially generated by gen_goto2.go (add link), as a start\n"
	license += "// of the Traffic Ops golang data model\n\n"

	header := "package " + config.PkgName + "\n\n"
	header += "import (\n"
	header += "\"fmt\"\n"

	sString := structString(schemas, table)

	if strings.Contains(sString, "null.") {
		header += "\"gopkg.in/guregu/null.v3\"\n"
	}
	header += "\"github.com/Comcast/traffic_control/traffic_ops/goto2/db\"\n"
	if strings.Contains(sString, "time.") {
		header += "\"time\"\n"
	}
	header += "\"encoding/json\"\n"
	header += ")\n\n"

	hString := handleString(schemas, table)
	totalBytes, err := fmt.Fprint(file, license+header+sString+hString)
	if err != nil {
		log.Fatal(err)
	}
	return totalBytes, nil
}

// gen a list of columnames without id and last_updated
func colString(schemas []ColumnSchema, table string, prefix string, varName string) string {
	out := ""
	sep := ""
	for _, cs := range schemas {
		if cs.TableName == table && cs.ColumnName != "id" && cs.ColumnName != "last_updated" {
			out += varName + "+= \"" + sep + prefix + cs.ColumnName + "\"\n"
			sep = ","
		}
	}
	return out
}

func genInsertVarLines(schemas []ColumnSchema, table string) string {
	out := "sqlString := \"INSERT INTO " + table + "(\"\n"
	out += colString(schemas, table, "", "sqlString")
	out += "sqlString += \") VALUES (\"\n"
	out += colString(schemas, table, ":", "sqlString")
	out += "sqlString += \")\"\n"

	return out
}

func updString(schemas []ColumnSchema, table string, prefix string, varName string) string {
	out := ""
	sep := ""
	for _, cs := range schemas {
		if cs.TableName == table && cs.ColumnName != "id" {
			out += varName + "+= \"" + sep + cs.ColumnName + " = :" + cs.ColumnName + "\"\n"
			sep = ","
		}
	}
	return out
}

func genUpdateVarLines(schemas []ColumnSchema, table string, whereCol string) string {
	out := "sqlString := \"UPDATE " + table + " SET \"\n"
	out += updString(schemas, table, "", "sqlString")
	out += "sqlString += \" WHERE " + whereCol + "=:" + whereCol + "\"\n"

	return out
}

func hasLastUpdated(schemas []ColumnSchema, table string) bool {
	for _, cs := range schemas {
		if cs.TableName == table {
			if cs.ColumnName == "last_updated" {
				return true
			}
		}
	}
	return false
}

func handleString(schemas []ColumnSchema, table string) string {
	// out := "func handle" + formatName(table) + "()([]" + formatName(table) + ", error) {\n"
	out := "func handle" + formatName(table) + "(method string, id int, payload []byte)(interface{}, error) {\n"
	out += "    if method == \"GET\" {\n"
	out += "        return get" + formatName(table) + "(id)\n"
	out += "   	} else if method == \"POST\" {\n"
	out += "        return post" + formatName(table) + "(payload)\n"
	out += "    } else if method == \"PUT\" {\n"
	out += "        return put" + formatName(table) + "(id, payload)\n"
	out += "    } else if method == \"DELETE\" {\n"
	out += "        return del" + formatName(table) + "(id)\n"
	out += "    }\n"
	out += "    return nil, nil\n"
	out += "}\n\n"

	// 	arg := TmUser{Username: null.StringFrom(username)}
	// nstmt, err := db.GlobalDB.PrepareNamed(`select * from tm_user where username=:username`)
	// err = nstmt.Select(&ret, arg)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// if len(ret) != 1 {
	// 	err := errors.New("Username " + username + " is not unique!")
	// }
	// nstmt.Close()

	idColumn := idCol(schemas, table)
	updateLastUpdated := hasLastUpdated(schemas, table)
	out += "func get" + formatName(table) + "(id int) (interface{}, error) {\n"
	out += "    ret := []" + formatName(table) + "{}\n"
	out += "    arg := " + formatName(table) + "{" + formatName(idColumn) + ": int64(id)}\n"
	out += "    if id >= 0 {\n"
	out += "        nstmt, err := db.GlobalDB.PrepareNamed(`select * from " + table + " where " + idColumn + "=:" + idColumn + "`)\n"
	out += "        err = nstmt.Select(&ret, arg)\n"
	out += "	    if err != nil {\n"
	out += "	        fmt.Println(err)\n"
	out += "	        return nil, err\n"
	out += "	    }\n"
	out += "        nstmt.Close()\n"
	out += "	} else {\n"
	out += "		queryStr := \"select * from " + table + "\"\n"
	out += "	    err := db.GlobalDB.Select(&ret, queryStr)\n"
	out += "	    if err != nil {\n"
	out += "		    fmt.Println(err)\n"
	out += "		    return nil, err\n"
	out += "	    }\n"
	out += "    }\n"
	out += "	return ret, nil\n"
	out += "}\n\n"

	out += "func post" + formatName(table) + "(payload []byte) (interface{}, error) {\n"
	out += "	var v " + formatName(table) + "\n"
	out += "	err := json.Unmarshal(payload, &v)\n"
	out += "	if err != nil {\n"
	out += "		fmt.Println(err)\n"
	out += "	}\n"
	out += genInsertVarLines(schemas, table)
	out += "    result, err := db.GlobalDB.NamedExec(sqlString, v)\n"
	out += "    if err != nil {\n"
	out += "        fmt.Println(err)\n"
	out += "    	return nil, err\n"
	out += "    }\n"
	out += "    return result, err\n"
	out += "}\n\n"

	out += "func put" + formatName(table) + "(id int, payload []byte) (interface{}, error) {\n"
	out += "    var v " + formatName(table) + "\n"
	out += "    err := json.Unmarshal(payload, &v)\n"
	out += "    v." + formatName(idColumn) + "= int64(id) // overwrite the id in the payload\n"
	out += "    if err != nil {\n"
	out += "    	fmt.Println(err)\n"
	out += "    	return nil, err\n"
	out += "    }\n"
	if updateLastUpdated {
		out += "    v.LastUpdated = time.Now()\n"
	}
	out += genUpdateVarLines(schemas, table, idColumn)
	out += "    result, err := db.GlobalDB.NamedExec(sqlString, v)\n"
	out += "    if err != nil {\n"
	out += "    	fmt.Println(err)\n"
	out += "    	return nil, err\n"
	out += "    }\n"
	out += "    return result, err\n"
	out += "}\n\n"

	out += "func del" + formatName(table) + "(id int) (interface{}, error) {\n"
	out += "    arg := " + formatName(table) + "{" + formatName(idColumn) + ": int64(id)}\n"
	out += "    result, err := db.GlobalDB.NamedExec(\"DELETE FROM " + table + " WHERE id=:id\", arg)\n"
	out += "    if err != nil {\n"
	out += "    	fmt.Println(err)\n"
	out += "    	return nil, err\n"
	out += "    }\n"
	out += "    return result, err\n"
	out += "}\n\n"
	return out
}

func structString(schemas []ColumnSchema, table string) string {

	out := "type " + formatName(table) + " struct{\n"
	for _, cs := range schemas {
		if cs.TableName == table {
			goType, _, err := goType(&cs)

			if err != nil {
				log.Fatal(err)
			}
			out = out + "\t" + formatName(cs.ColumnName) + " " + goType
			if len(config.TagLabel) > 0 {
				out = out + "\t`" + config.TagLabel + ":\"" + cs.ColumnName + "\" json:\"" + formatNameLower(cs.ColumnName) + "\"`"
			}
			out = out + "\n"
		}
	}
	out = out + "}\n\n"

	return out
}

func getSchema() ([]ColumnSchema, []string) {
	conn, err := sql.Open("mysql", config.DbUser+":"+config.DbPassword+"@/information_schema")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	q := "SELECT TABLE_NAME, COLUMN_NAME, IS_NULLABLE, DATA_TYPE, " +
		"CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, COLUMN_TYPE, " +
		"COLUMN_KEY FROM COLUMNS WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME, ORDINAL_POSITION"
	rows, err := conn.Query(q, config.DbName)
	if err != nil {
		log.Fatal(err)
	}
	columns := []ColumnSchema{}
	for rows.Next() {
		cs := ColumnSchema{}
		err := rows.Scan(&cs.TableName, &cs.ColumnName, &cs.IsNullable, &cs.DataType,
			&cs.CharacterMaximumLength, &cs.NumericPrecision, &cs.NumericScale,
			&cs.ColumnType, &cs.ColumnKey)
		if err != nil {
			log.Fatal(err)
		}
		columns = append(columns, cs)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	q = "select TABLE_NAME from tables WHERE TABLE_SCHEMA = ? AND table_type='BASE TABLE'"
	rows, err = conn.Query(q, config.DbName)
	if err != nil {
		log.Fatal(err)
	}
	tables := []string{}
	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		if err != nil {
			log.Fatal(err)
		}
		tables = append(tables, tableName)
	}
	return columns, tables
}

func formatName(name string) string {
	parts := strings.Split(name, "_")
	newName := ""
	for _, p := range parts {
		if len(p) < 1 {
			continue
		}
		newName = newName + strings.Replace(p, string(p[0]), strings.ToUpper(string(p[0])), 1)
	}
	return newName
}

func formatNameLower(name string) string {
	newName := formatName(name)
	newName = strings.Replace(newName, string(newName[0]), strings.ToLower(string(newName[0])), 1)
	return newName
}

func goType(col *ColumnSchema) (string, string, error) {
	requiredImport := ""
	if col.IsNullable == "YES" {
		requiredImport = "database/sql"
	}
	var gt string = ""
	switch col.DataType {
	case "char", "varchar", "enum", "text", "longtext", "mediumtext", "tinytext":
		if col.IsNullable == "YES" {
			gt = "null.String"
		} else {
			gt = "string"
		}
	case "blob", "mediumblob", "longblob", "varbinary", "binary":
		gt = "[]byte"
	case "date", "time", "datetime", "timestamp":
		gt, requiredImport = "time.Time", "time"
	case "tinyint", "smallint", "int", "mediumint", "bigint":
		if col.IsNullable == "YES" {
			gt = "null.Int"
		} else {
			gt = "int64"
		}
	case "float", "decimal", "double":
		if col.IsNullable == "YES" {
			gt = "null.Float"
		} else {
			gt = "float64"
		}
	}
	if gt == "" {
		n := col.TableName + "." + col.ColumnName
		return "", "", errors.New("No compatible datatype (" + col.DataType + ") for " + n + " found")
	}
	return gt, requiredImport, nil
}

func main() {

	config = defaults

	columns, tables := getSchema()
	fmt.Println(tables)
	for _, table := range tables {
		bytes, err := writeFile(columns, table)
		if err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command("go", "fmt", "./generated/"+table+".go")
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: Ok %d\n", table, bytes)
	}
}
