package bossq

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ModelInfo хранит разобранную информацию о модели.
type ModelInfo struct {
	Package   string
	ModelName string
	StoreName string
	TableName string
	Fields    []FieldInfo
	PKField   string
	PKColumn  string
	Imports   []string
}

// FieldInfo описывает одно поле модели.
type FieldInfo struct {
	Name       string
	Type       string
	ColumnName string
	IsPK       bool
	IsAutoinc  bool
	IsNotNull  bool
	HasDefault bool
	DefaultVal string
	Tag        string
}

// Generate читает все Go-файлы в директории, находит модели с тегом bossq
// и генерирует для каждой файл {name}_bossq.go.
func Generate(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse dir %s: %w", dir, err)
	}

	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			models := extractModels(file)
			for _, model := range models {
				model.Package = pkg.Name
				if err := writeStoreFile(fileName, model); err != nil {
					return fmt.Errorf("write store for %s: %w", model.ModelName, err)
				}
			}
		}
	}
	return nil
}

// extractModels проходит по AST-файлу и собирает все структуры с тегом bossq.
func extractModels(file *ast.File) []ModelInfo {
	var models []ModelInfo

	ast.Inspect(file, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		tableName := ""
		if typeSpec.Comment != nil {
			for _, comment := range typeSpec.Comment.List {
				text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
				if strings.HasPrefix(text, "bossq:table=") {
					tableName = strings.TrimPrefix(text, "bossq:table=")
					break
				}
			}
		}
		if tableName == "" {
			return true
		}

		model := ModelInfo{
			ModelName: typeSpec.Name.Name,
			StoreName: typeSpec.Name.Name + "Store",
			TableName: tableName,
		}

		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}
			fInfo := FieldInfo{
				Name: field.Names[0].Name,
				Type: typeToString(field.Type),
			}

			if field.Tag != nil {
				tag := strings.Trim(field.Tag.Value, "`")
				fInfo.Tag = tag
				fInfo.parseTag(tag)
			}

			// Имя колонки по умолчанию = snake_case имени поля
			if fInfo.ColumnName == "" {
				fInfo.ColumnName = toSnakeCase(fInfo.Name)
			}

			if fInfo.IsPK {
				model.PKField = fInfo.Name
				model.PKColumn = fInfo.ColumnName
			}
			model.Fields = append(model.Fields, fInfo)
		}

		models = append(models, model)
		return true
	})

	return models
}

// writeStoreFile генерирует файл *_bossq.go для одной модели.
func writeStoreFile(originalFile string, model ModelInfo) error {
	tmpl, err := template.New("store").Parse(storeTemplate)
	if err != nil {
		return err
	}

	// Генерируем методы
	var methods []string

	if create, err := genCreateMethod(model); err == nil {
		methods = append(methods, create)
	}
	if getByID, err := genGetByIDMethod(model); err == nil {
		methods = append(methods, getByID)
	}
	if update, err := genUpdateMethod(model); err == nil {
		methods = append(methods, update)
	}
	if deleteM, err := genDeleteMethod(model); err == nil {
		methods = append(methods, deleteM)
	}
	if list, err := genListMethod(model); err == nil {
		methods = append(methods, list)
	}
	if bulk, err := genBulkInsertMethod(model); err == nil {
		methods = append(methods, bulk)
	}
	if qbCode, err := genQueryBuilder(model); err == nil {
		methods = append(methods, qbCode)
	}

	data := struct {
		Package   string
		StoreName string
		TableName string
		Methods   []string
	}{
		Package:   model.Package,
		StoreName: model.StoreName,
		TableName: model.TableName,
		Methods:   methods,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	// Имя выходного файла: models.go -> models_bossq.go
	base := strings.TrimSuffix(filepath.Base(originalFile), ".go")
	outFileName := filepath.Join(filepath.Dir(originalFile), base+"_bossq.go")
	return os.WriteFile(outFileName, buf.Bytes(), 0644)
}

// ====================== Вспомогательные генераторы методов ======================

func genCreateMethod(m ModelInfo) (string, error) {
	var insertCols, insertPlaceholders, returningCols, insertVals, scanReturning, copyBack []string
	idx := 0
	for _, f := range m.Fields {
		if f.IsAutoinc {
			returningCols = append(returningCols, f.ColumnName)
			scanReturning = append(scanReturning, "&newM."+f.Name)
			continue
		}
		idx++
		insertCols = append(insertCols, f.ColumnName)
		insertPlaceholders = append(insertPlaceholders, fmt.Sprintf("$%d", idx))
		insertVals = append(insertVals, "m."+f.Name)
		copyBack = append(copyBack, fmt.Sprintf("newM.%s = m.%s", f.Name, f.Name))
	}

	data := map[string]string{
		"StoreName":          m.StoreName,
		"ModelName":          m.ModelName,
		"TableName":          m.TableName,
		"InsertColumns":      strings.Join(insertCols, ", "),
		"InsertPlaceholders": strings.Join(insertPlaceholders, ", "),
		"ReturningColumns":   strings.Join(returningCols, ", "),
		"InsertValues":       strings.Join(insertVals, ", "),
		"ScanReturning":      strings.Join(scanReturning, ", "),
		"CopyBackFields":     strings.Join(copyBack, "\n\t"),
	}

	tmpl, err := template.New("create").Parse(createMethod)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genGetByIDMethod(m ModelInfo) (string, error) {
	var allCols, scanAll []string
	for _, f := range m.Fields {
		allCols = append(allCols, f.ColumnName)
		scanAll = append(scanAll, "&m."+f.Name)
	}

	data := map[string]string{
		"StoreName":  m.StoreName,
		"ModelName":  m.ModelName,
		"TableName":  m.TableName,
		"PKColumn":   m.PKColumn,
		"AllColumns": strings.Join(allCols, ", "),
		"ScanAll":    strings.Join(scanAll, ", "),
	}

	tmpl, _ := template.New("getbyid").Parse(getByIDMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String(), nil
}

func genUpdateMethod(m ModelInfo) (string, error) {
	var setClauses, updateVals []string
	idx := 0
	for _, f := range m.Fields {
		if f.IsPK {
			continue
		}
		idx++
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", f.ColumnName, idx))
		updateVals = append(updateVals, "m."+f.Name)
	}
	idx++
	pkIdx := idx
	updateVals = append(updateVals, "m."+m.PKField)

	data := map[string]string{
		"StoreName":    m.StoreName,
		"ModelName":    m.ModelName,
		"TableName":    m.TableName,
		"PKColumn":     m.PKColumn,
		"PKField":      m.PKField,
		"PKIndex":      fmt.Sprintf("%d", pkIdx),
		"SetClauses":   strings.Join(setClauses, ", "),
		"UpdateValues": strings.Join(updateVals, ", "),
	}

	tmpl, _ := template.New("update").Parse(updateMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String(), nil
}

func genDeleteMethod(m ModelInfo) (string, error) {
	tmpl, _ := template.New("delete").Parse(deleteMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, map[string]string{
		"StoreName": m.StoreName,
		"TableName": m.TableName,
		"PKColumn":  m.PKColumn,
	})
	return buf.String(), nil
}

func genListMethod(m ModelInfo) (string, error) {
	var allCols, scanAll []string
	for _, f := range m.Fields {
		allCols = append(allCols, f.ColumnName)
		scanAll = append(scanAll, "&m."+f.Name)
	}

	data := map[string]string{
		"StoreName":  m.StoreName,
		"ModelName":  m.ModelName,
		"TableName":  m.TableName,
		"PKColumn":   m.PKColumn,
		"AllColumns": strings.Join(allCols, ", "),
		"ScanAll":    strings.Join(scanAll, ", "),
	}

	tmpl, _ := template.New("list").Parse(listMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String(), nil
}

func genBulkInsertMethod(m ModelInfo) (string, error) {
	var colNames, copyVals []string
	for _, f := range m.Fields {
		if f.IsAutoinc {
			continue
		}
		colNames = append(colNames, `"`+f.ColumnName+`"`)
		copyVals = append(copyVals, "m."+f.Name)
	}

	data := map[string]string{
		"StoreName":          m.StoreName,
		"ModelName":          m.ModelName,
		"TableName":          m.TableName,
		"ColumnNamesForCopy": strings.Join(colNames, ", "),
		"CopyValues":         strings.Join(copyVals, ", "),
	}

	tmpl, _ := template.New("bulkinsert").Parse(bulkInsertMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String(), nil
}

// ====================== Утилиты ======================

// typeToString преобразует ast.Expr в строку типа Go.
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// toSnakeCase переводит CamelCase в snake_case.
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+32))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// parseTag разбирает тег bossq.
func (f *FieldInfo) parseTag(tag string) {
	// Ищем значение bossq:"..."
	start := strings.Index(tag, `bossq:"`)
	if start == -1 {
		return
	}
	start += len(`bossq:"`)
	end := strings.Index(tag[start:], `"`)
	if end == -1 {
		return
	}
	value := tag[start : start+end]

	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "pk":
			f.IsPK = true
		case part == "autoinc":
			f.IsAutoinc = true
		case part == "notnull":
			f.IsNotNull = true
		case strings.HasPrefix(part, "type="):
			// учитываем, но не обрабатываем здесь
		case strings.HasPrefix(part, "default="):
			f.HasDefault = true
			f.DefaultVal = strings.TrimPrefix(part, "default=")
		case strings.HasPrefix(part, "column="):
			f.ColumnName = strings.TrimPrefix(part, "column=")
		}
	}
}

func genQueryBuilder(m ModelInfo) (string, error) {
    // Генерируем методы для каждого поля
    var fieldMethods []string
    for _, f := range m.Fields {
        data := map[string]string{
            "ModelName":  m.ModelName,
            "FieldName":  f.Name,
            "FieldType":  f.Type,
            "ColumnName": f.ColumnName,
        }
        // Генерируем Where и AndWhere
        tmpl, _ := template.New("fieldWhere").Parse(fieldWhereMethod)
        var buf bytes.Buffer
        if err := tmpl.Execute(&buf, data); err != nil {
            return "", err
        }
        fieldMethods = append(fieldMethods, buf.String())
    }

    // Собираем полный код QueryBuilder
    tmpl, err := template.New("queryBuilder").Parse(queryBuilderTemplate)
    if err != nil {
        return "", err
    }
    // Сформируем список всех полей для Scan (используем тот же ScanAll, что и в GetByID)
    var allCols []string
    for _, f := range m.Fields {
        allCols = append(allCols, f.ColumnName)
    }
    scanAll := make([]string, len(m.Fields))
    for i, f := range m.Fields {
        scanAll[i] = "&m." + f.Name
    }

    data := struct {
        ModelName    string
        TableName    string
        FieldMethods []string
        ScanAll      string
    }{
        ModelName:    m.ModelName,
        TableName:    m.TableName,
        FieldMethods: fieldMethods,
        ScanAll:      strings.Join(scanAll, ", "),
    }
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Выполнено с любовью для Босса 🐈‍