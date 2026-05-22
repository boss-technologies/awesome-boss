package bossq

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/boss-technologies/awesome-boss/bossq"

	"golang.org/x/tools/go/packages"
)

var allowedWhereTypes = map[string]bool{
	"int":             true,
	"int64":           true,
	"uint":            true,
	"uint64":          true,
	"float32":         true,
	"float64":         true,
	"string":          true,
	"bool":            true,
	"time.Time":       true,
	"decimal.Decimal": true,
}

// Generate читает все Go-файлы в директории, находит модели с тегом bossq
// и генерирует для каждой файл {name}_bossq.go.
func Generate(dir string) error {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return fmt.Errorf("packages.Load: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("в пакете %s есть ошибки компиляции", dir)
	}

	var allModels []bossq.ModelInfo
	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			models := extractModels(file)
			for j := range models {
				models[j].Package = pkg.Name
				models[j].SourceFile = pkg.GoFiles[i]
			}
			allModels = append(allModels, models...)
		}
	}

	if len(allModels) == 0 {
		return nil
	}

	modelMap := make(map[string]bossq.ModelInfo, len(allModels))
	for _, m := range allModels {
		modelMap[m.ModelName] = m
	}

	for _, model := range allModels {
		if err := writeStoreFile(model.SourceFile, model, modelMap); err != nil {
			return fmt.Errorf("write store for %s: %w", model.ModelName, err)
		}
	}
	return nil
}

// extractModels проходит по AST-файлу и собирает все структуры с тегом bossq.
func extractModels(file *ast.File) []bossq.ModelInfo {
	var models []bossq.ModelInfo

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		tableName := ""
		ftsLanguage := ""
		if genDecl.Doc != nil {
			for _, comment := range genDecl.Doc.List {
				text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
				if after, found := strings.CutPrefix(text, "bossq:table="); found {
					tableName = after
				}
				if after, found := strings.CutPrefix(text, "bossq:fts="); found {
					ftsLanguage = after
				}
			}
		}

		if tableName == "" {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			localTable := tableName
			localFTS := ftsLanguage
			if typeSpec.Doc != nil {
				for _, comment := range typeSpec.Doc.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if after, found := strings.CutPrefix(text, "bossq:table="); found {
						localTable = after
					}
					if after, found := strings.CutPrefix(text, "bossq:fts="); found {
						localFTS = after
					}
				}
			} else if typeSpec.Comment != nil {
				for _, comment := range typeSpec.Comment.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if after, found := strings.CutPrefix(text, "bossq:table="); found {
						localTable = after
					}
					if after, found := strings.CutPrefix(text, "bossq:fts="); found {
						localFTS = after
					}
				}
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			model := bossq.ModelInfo{
				ModelName:   typeSpec.Name.Name,
				StoreName:   typeSpec.Name.Name + "Store",
				TableName:   localTable,
				FTSLanguage: localFTS,
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				fInfo := bossq.FieldInfo{
					Name: field.Names[0].Name,
					Type: typeToString(field.Type),
				}
				if field.Tag != nil {
					tag := strings.Trim(field.Tag.Value, "`")
					relTag := extractRelationTag(tag)
					if relTag.Type != "" {
						relTag.Name = field.Names[0].Name
						model.Relations = append(model.Relations, relTag)
						continue
					}
					fInfo.Tag = tag
					fInfo.ParseTag(tag)
				}

				if fInfo.ColumnName == "" {
					fInfo.ColumnName = toSnakeCase(fInfo.Name)
				}

				if fInfo.IsPK {
					model.PKField = fInfo.Name
					model.PKColumn = fInfo.ColumnName
				}

				model.Fields = append(model.Fields, fInfo)

				if fInfo.FTSWeight != "" {
					model.FTSFields = append(model.FTSFields, bossq.FTSFieldInfo{
						FieldName:  fInfo.Name,
						ColumnName: fInfo.ColumnName,
						Weight:     fInfo.FTSWeight,
					})
				}
			}

			for _, f := range model.Fields {
				if f.IsUnique {
					model.UniqueFields = append(model.UniqueFields, f)
				}
			}

			models = append(models, model)
		}
	}

	return models
}

// writeStoreFile генерирует файл *_bossq.go для одной модели.
func writeStoreFile(originalFile string, model bossq.ModelInfo, allModels map[string]bossq.ModelInfo) error {
	tmpl, err := template.New("store").Parse(storeTemplate)
	if err != nil {
		return err
	}

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
	if listPaginated, err := genListPaginatedMethod(model); err == nil {
		methods = append(methods, listPaginated)
	}
	relMethods, err := genRelationMethods(model, allModels)
	if err != nil {
		return err
	}
	methods = append(methods, relMethods...)

	for _, uf := range model.UniqueFields {
		getMethod, err := genGetByUniqueMethod(model, uf)
		if err != nil {
			return err
		}
		methods = append(methods, getMethod)
	}

	if model.FTSLanguage != "" && len(model.FTSFields) > 0 {
		if search, err := genSearchMethod(model); err == nil {
			methods = append(methods, search)
		}
		if headline, err := genSearchWithHeadlineMethod(model); err == nil {
			methods = append(methods, headline)
		}
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

	base := strings.TrimSuffix(filepath.Base(originalFile), ".go")
	outFileName := filepath.Join(filepath.Dir(originalFile), base+"_bossq.go")
	return os.WriteFile(outFileName, buf.Bytes(), 0644)
}

// ---------------------- Методы ----------------------

// genCreateMethod генерирует Create: с RETURNING если есть autoinc, иначе простой INSERT.
func genCreateMethod(m bossq.ModelInfo) (string, error) {
	var insertCols, insertPlaceholders, insertVals []string
	var returningCols, scanReturning []string
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
	}

	if len(returningCols) > 0 {
		// Есть автоинкрементные поля – используем RETURNING
		data := map[string]string{
			"StoreName":          m.StoreName,
			"ModelName":          m.ModelName,
			"TableName":          m.TableName,
			"InsertColumns":      strings.Join(insertCols, ", "),
			"InsertPlaceholders": strings.Join(insertPlaceholders, ", "),
			"ReturningColumns":   strings.Join(returningCols, ", "),
			"InsertValues":       strings.Join(insertVals, ", "),
			"ScanReturning":      strings.Join(scanReturning, ", "),
		}
		tmpl, err := template.New("createReturning").Parse(createReturningMethod)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}

	// Нет автоинкремента – простой INSERT, возвращаем переданную модель
	data := map[string]string{
		"StoreName":          m.StoreName,
		"ModelName":          m.ModelName,
		"TableName":          m.TableName,
		"InsertColumns":      strings.Join(insertCols, ", "),
		"InsertPlaceholders": strings.Join(insertPlaceholders, ", "),
		"InsertValues":       strings.Join(insertVals, ", "),
	}
	tmpl, err := template.New("createSimple").Parse(createSimpleMethod)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genGetByIDMethod(m bossq.ModelInfo) (string, error) {
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

func genGetByUniqueMethod(m bossq.ModelInfo, field bossq.FieldInfo) (string, error) {
	allCols := make([]string, len(m.Fields))
	scanAll := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		allCols[i] = f.ColumnName
		scanAll[i] = "&m." + f.Name
	}
	data := map[string]string{
		"StoreName":  m.StoreName,
		"ModelName":  m.ModelName,
		"TableName":  m.TableName,
		"FieldName":  field.Name,
		"ColumnName": field.ColumnName,
		"FieldType":  field.Type,
		"AllColumns": strings.Join(allCols, ", "),
		"ScanAll":    strings.Join(scanAll, ", "),
	}
	tmpl, err := template.New("getByUnique").Parse(getByUniqueMethodTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genListPaginatedMethod(m bossq.ModelInfo) (string, error) {
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
	tmpl, _ := template.New("listPaginated").Parse(listPaginatedMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String(), nil
}

func genUpdateMethod(m bossq.ModelInfo) (string, error) {
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

func genDeleteMethod(m bossq.ModelInfo) (string, error) {
	tmpl, _ := template.New("delete").Parse(deleteMethod)
	var buf bytes.Buffer
	tmpl.Execute(&buf, map[string]string{
		"StoreName": m.StoreName,
		"TableName": m.TableName,
		"PKColumn":  m.PKColumn,
	})
	return buf.String(), nil
}

func genListMethod(m bossq.ModelInfo) (string, error) {
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

func genBulkInsertMethod(m bossq.ModelInfo) (string, error) {
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

// ---------------------- QueryBuilder ----------------------

func genQueryBuilder(m bossq.ModelInfo) (string, error) {
	var fieldMethods []string
	for _, f := range m.Fields {
		if !allowedWhereTypes[f.Type] {
			continue
		}
		data := map[string]string{
			"ModelName":  m.ModelName,
			"FieldName":  f.Name,
			"FieldType":  f.Type,
			"ColumnName": f.ColumnName,
		}
		tmpl, _ := template.New("fieldWhere").Parse(fieldWhereMethod) // теперь только Where, без AndWhere
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", err
		}
		fieldMethods = append(fieldMethods, buf.String())
	}

	tmpl, err := template.New("queryBuilder").Parse(queryBuilderTemplate)
	if err != nil {
		return "", err
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
// ---------------------- Связи ----------------------
func extractRelationTag(tag string) bossq.RelationInfo {
	start := strings.Index(tag, `bossq:"`)
	if start == -1 {
		return bossq.RelationInfo{}
	}
	start += len(`bossq:"`)
	end := strings.Index(tag[start:], `"`)
	if end == -1 {
		return bossq.RelationInfo{}
	}
	value := tag[start : start+end]

	// hasMany:Model.ForeignKey
	if strings.HasPrefix(value, "hasMany:") {
		parts := strings.Split(value[len("hasMany:"):], ".")
		if len(parts) == 2 {
			return bossq.RelationInfo{
				Type:       "HasMany",
				Model:      parts[0],
				ForeignKey: parts[1],
			}
		}
	}

	// belongsTo:Model.ForeignKey
	if strings.HasPrefix(value, "belongsTo:") {
		parts := strings.Split(value[len("belongsTo:"):], ".")
		if len(parts) == 2 {
			return bossq.RelationInfo{
				Type:    "BelongsTo",
				Model:   parts[0],
				FKField: parts[1],
			}
		}
	}

	return bossq.RelationInfo{}
}

func genRelationMethods(model bossq.ModelInfo, allModels map[string]bossq.ModelInfo) ([]string, error) {
	var methods []string
	for _, rel := range model.Relations {
		if rel.FKField == "" && rel.Type == "BelongsTo" {
			return nil, fmt.Errorf("для связи BelongsTo в модели %s поле FKField не указано (используйте формат belongsTo:Model.Field)", model.ModelName)
		}
		targetModel, ok := allModels[rel.Model]
		if !ok {
			return nil, fmt.Errorf("модель %q не найдена для связи %q", rel.Model, rel.Name)
		}
		switch rel.Type {
		case "HasMany":
			code, err := genHasManyMethod(model, rel, targetModel)
			if err != nil {
				return nil, err
			}
			methods = append(methods, code)
		case "BelongsTo":
			code, err := genBelongsToMethod(model, rel, targetModel)
			if err != nil {
				return nil, err
			}
			methods = append(methods, code)
		}
	}
	return methods, nil
}

func genHasManyMethod(parent bossq.ModelInfo, rel bossq.RelationInfo, target bossq.ModelInfo) (string, error) {
	var scanFields []string
	for _, f := range target.Fields {
		scanFields = append(scanFields, "&item."+f.Name)
	}
	data := map[string]string{
		"StoreName":    parent.StoreName,
		"ModelName":    parent.ModelName,
		"RelationName": rel.Name,
		"TargetTable":  target.TableName,
		"ForeignKey":   rel.ForeignKey,
		"PKField":      parent.PKField,
		"ScanFields":   strings.Join(scanFields, ", "),
		"FieldType":    "[]" + rel.Model,
	}
	tmpl, err := template.New("hasmany").Parse(loadHasManyTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genBelongsToMethod(parent bossq.ModelInfo, rel bossq.RelationInfo, target bossq.ModelInfo) (string, error) {
	var scanFields []string
	for _, f := range target.Fields {
		scanFields = append(scanFields, "&item."+f.Name)
	}
	data := map[string]string{
		"StoreName":    parent.StoreName,
		"ModelName":    parent.ModelName,
		"RelationName": rel.Name,
		"TargetTable":  target.TableName,
		"FKField":      rel.FKField,
		"TargetPK":     target.PKField,
		"ScanFields":   strings.Join(scanFields, ", "),
		"TargetModel":  rel.Model,
	}
	tmpl, err := template.New("belongsTo").Parse(loadBelongsToTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ---------------------- FTS ----------------------

func genSearchMethod(m bossq.ModelInfo) (string, error) {
	var allCols, scanAll []string
	for _, f := range m.Fields {
		allCols = append(allCols, f.ColumnName)
		scanAll = append(scanAll, "&m."+f.Name)
	}
	data := map[string]string{
		"StoreName":   m.StoreName,
		"ModelName":   m.ModelName,
		"TableName":   m.TableName,
		"FTSLanguage": m.FTSLanguage,
		"AllColumns":  strings.Join(allCols, ", "),
		"ScanAll":     strings.Join(scanAll, ", "),
	}
	tmpl, _ := template.New("search").Parse(searchMethod)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genSearchWithHeadlineMethod(m bossq.ModelInfo) (string, error) {
	var selectParts, scanParts []string
	type fieldStruct struct{ Name, Type string }
	var fieldsForStruct []fieldStruct

	for _, f := range m.Fields {
		selectParts = append(selectParts, f.ColumnName)
		scanParts = append(scanParts, "&item."+f.Name)
		fieldsForStruct = append(fieldsForStruct, fieldStruct{f.Name, f.Type})
	}

	headlineField := m.FTSFields[0].ColumnName
	selectWithHeadline := strings.Join(selectParts, ", ") +
		fmt.Sprintf(`, ts_headline('%s', %s, plainto_tsquery('%s', $2), 'MaxWords=30, MinWords=15') AS headline`,
			m.FTSLanguage, headlineField, m.FTSLanguage)
	scanHeadline := strings.Join(scanParts, ", ") + ", &item.Headline"

	data := struct {
		StoreName          string
		ModelName          string
		TableName          string
		FTSLanguage        string
		SelectWithHeadline string
		ScanHeadline       string
		Fields             []fieldStruct
	}{
		StoreName:          m.StoreName,
		ModelName:          m.ModelName,
		TableName:          m.TableName,
		FTSLanguage:        m.FTSLanguage,
		SelectWithHeadline: selectWithHeadline,
		ScanHeadline:       scanHeadline,
		Fields:             fieldsForStruct,
	}
	tmpl, err := template.New("searchHeadline").Parse(searchWithHeadlineMethod)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ---------------------- Утилиты ----------------------

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

// toSnakeCase обрабатывает CamelCase и аббревиатуры.
func toSnakeCase(s string) string {
	// Список часто встречающихся аббревиатур, которые должны оставаться в нижнем регистре без разбиения.
	abbreviations := map[string]string{
		"ID":  "id",
		"URL": "url",
		"URI": "uri",
		"HTTP": "http",
		"HTTPS": "https",
		"API": "api",
		"JSON": "json",
		"XML": "xml",
		"SQL": "sql",
		"UUID": "uuid",
		"JWT": "jwt",
		"CSV": "csv",
		"HTML": "html",
		"CSS": "css",
		"JS": "js",
		"PDF": "pdf",
		"TXT": "txt",
		"ZIP": "zip",
		"RAR": "rar",
		"EXE": "exe",
		"BIN": "bin",
		"IMG": "img",
		"PNG": "png",
		"JPG": "jpg",
		"JPEG": "jpeg",
		"GIF": "gif",
		"SVG": "svg",
		"MP3": "mp3",
		"MP4": "mp4",
		"AVI": "avi",
		"MKV": "mkv",
		"MOV": "mov",
		"WAV": "wav",
		"FLAC": "flac",
		"OGG": "ogg",
		"WEBM": "webm",
		"WEBP": "webp",
		"BMP": "bmp",
		"ICO": "ico",
		"TIF": "tif",
		"TIFF": "tiff",
		"PSD": "psd",
		"AI": "ai",
		"EPS": "eps",
		"INDD": "indd",
		"RAW": "raw",
		"CR2": "cr2",
		"NEF": "nef",
		"ORF": "orf",
		"SRW": "srw",
		"ARW": "arw",
		"DNG": "dng",
		"MRW": "mrw",
		"PEF": "pef",
		"RAF": "raf",
		"RW2": "rw2",
		"X3F": "x3f",
		"3FR": "3fr",
		"FFF": "fff",
		"DCR": "dcr",
		"KDC": "kdc",
		"MEF": "mef",
		"MOS": "mos",
		"NRW": "nrw",
		"RWL": "rwl",
		"SR2": "sr2",
		"SRF": "srf",
		"XMF": "xmf",
		"ERF": "erf",
		"IIQ": "iiq",
	}

	// Прямое совпадение
	if lower, ok := abbreviations[s]; ok {
		return lower
	}

	// Общая логика для CamelCase с поддержкой аббревиатур на лету (простая)
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			// Проверяем, не начинается ли здесь аббревиатура
			if i+1 < len(s) && s[i+1] >= 'A' && s[i+1] <= 'Z' {
				// Это часть аббревиатуры – пройдём до конца аббревиатуры
				j := i
				for j < len(s) && s[j] >= 'A' && s[j] <= 'Z' {
					j++
				}
				abbr := s[i:j]
				if lowerAbbr, ok := abbreviations[abbr]; ok {
					if i > 0 {
						result = append(result, '_')
					}
					result = append(result, lowerAbbr...)
					i = j - 1
					continue
				}
			}
			// Обычная заглавная буква
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, c+32)
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
// Выполнено с любовью для Босса 🐈‍