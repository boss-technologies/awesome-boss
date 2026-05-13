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

	"golang.org/x/tools/go/packages"
)

// ModelInfo хранит разобранную информацию о модели.
type ModelInfo struct {
	Package    string
	ModelName  string
	StoreName  string
	TableName  string
	Fields     []FieldInfo
	PKField    string
	PKColumn   string
	Relations  []RelationInfo
	Imports    []string
	SourceFile string // путь к исходному файлу, чтобы сгенерировать рядом
	// FTS
	FTSLanguage string         // язык полнотекстового поиска, например "ru_hunspell"
	FTSFields   []FTSFieldInfo // поля, участвующие в FTS с весами
}

// FTSFieldInfo описывает одно поле для полнотекстового поиска.
type FTSFieldInfo struct {
	FieldName  string
	ColumnName string
	Weight     string // A, B, C, D
}

// RelationInfo описывает одну связь модели.
type RelationInfo struct {
	Name       string
	Type       string
	Model      string
	ForeignKey string // внешний ключ в целевой таблице (для HasMany) или имя поля в текущей модели (для BelongsTo)
	References string // пока не используется
	FKField    string // имя поля внешнего ключа в текущей модели (для BelongsTo)
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
	FTSWeight  string // вес, если поле участвует в FTS
}

var allowedWhereTypes = map[string]bool{
	"int":            true,
	"int64":          true,
	"uint":           true,
	"uint64":         true,
	"float32":        true,
	"float64":        true,
	"string":         true,
	"bool":           true,
	"time.Time":      true,
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

	var allModels []ModelInfo
	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			models := extractModels(file)
			for j := range models {
				models[j].Package = pkg.Name
				// Используем реальный путь из pkg.GoFiles
				models[j].SourceFile = pkg.GoFiles[i]
			}
			allModels = append(allModels, models...)
		}
	}

	if len(allModels) == 0 {
		// Не ошибка: просто в этой папке нет моделей.
		return nil
	}

	// Карта моделей для связей.
	modelMap := make(map[string]ModelInfo, len(allModels))
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
func extractModels(file *ast.File) []ModelInfo {
	var models []ModelInfo

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		// Комментарий может быть у самого GenDecl (группа типов) или у отдельной спецификации.
		// Сначала смотрим в GenDecl.Doc (или GenDecl.Comment для висячего комментария).
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
		// Никакой проверки genDecl.Comment! Её здесь быть не должно.

		if tableName == "" {
			continue // это не наша модель, пропускаем группу
		}

		// Теперь обходим все спецификации типов внутри этой группы
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Если у конкретного типа есть свой Doc, он переопределяет групповой?
			// Для простоты, если у спецификации есть свой комментарий с table=, используем его.
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

			model := ModelInfo{
				ModelName:   typeSpec.Name.Name,
				StoreName:   typeSpec.Name.Name + "Store",
				TableName:   localTable,
				FTSLanguage: localFTS,
			}

			// Обход полей структуры
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				fInfo := FieldInfo{
					Name: field.Names[0].Name,
					Type: typeToString(field.Type),
				}

				// Обрабатываем тег
				if field.Tag != nil {
					tag := strings.Trim(field.Tag.Value, "`")
					// Сначала проверяем, не является ли поле связью
					relTag := extractRelationTag(tag)
					if relTag.Type != "" {
						relTag.Name = field.Names[0].Name
						model.Relations = append(model.Relations, relTag)
						continue // связь не попадает в список полей для SQL
					}
					// Обычное поле: парсим тег
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

				// Добавляем поле в модель
				model.Fields = append(model.Fields, fInfo)

				// Если поле имеет вес FTS, добавляем в список FTSFields
				if fInfo.FTSWeight != "" {
					model.FTSFields = append(model.FTSFields, FTSFieldInfo{
						FieldName:  fInfo.Name,
						ColumnName: fInfo.ColumnName,
						Weight:     fInfo.FTSWeight,
					})
				}
			}

			models = append(models, model)
		}
	}

	return models
}

// writeStoreFile генерирует файл *_bossq.go для одной модели.
func writeStoreFile(originalFile string, model ModelInfo, allModels map[string]ModelInfo) error {
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
	if listPaginated, err := genListPaginatedMethod(model); err == nil {
		methods = append(methods, listPaginated)
	}
	relMethods, err := genRelationMethods(model, allModels)
	if err != nil {
		return err
	}
	methods = append(methods, relMethods...)

	// Добавляем FTS методы, если задан язык и есть поля
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

func genListPaginatedMethod(m ModelInfo) (string, error) {
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

	parts := strings.SplitSeq(value, ",")
	for part := range parts {
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
		case strings.HasPrefix(part, "fts="):
			f.FTSWeight = strings.TrimPrefix(part, "fts=")
		}
	}
}

func genQueryBuilder(m ModelInfo) (string, error) {
	var fieldMethods []string
	for _, f := range m.Fields {
		// Пропускаем поля, тип которых не входит в белый список
		if !allowedWhereTypes[f.Type] {
			continue
		}
		data := map[string]string{
			"ModelName":  m.ModelName,
			"FieldName":  f.Name,
			"FieldType":  f.Type,
			"ColumnName": f.ColumnName,
		}
		tmpl, _ := template.New("fieldWhere").Parse(fieldWhereMethod)
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

func extractRelationTag(tag string) RelationInfo {
	start := strings.Index(tag, `bossq:"`)
	if start == -1 {
		return RelationInfo{}
	}
	start += len(`bossq:"`)
	end := strings.Index(tag[start:], `"`)
	if end == -1 {
		return RelationInfo{}
	}
	value := tag[start : start+end]

	if strings.HasPrefix(value, "hasMany:") {
		parts := strings.Split(value[len("hasMany:"):], ".")
		if len(parts) == 2 {
			return RelationInfo{
				Type:       "HasMany",
				Model:      parts[0],
				ForeignKey: parts[1], // Book.AuthorID -> ForeignKey = AuthorID
			}
		}
	} else if strings.HasPrefix(value, "belongsTo:") {
		// Разбираем модель и опциональный fk
		rest := value[len("belongsTo:"):]
		// Проверяем наличие скобок
		modelName := rest
		fkField := ""
		if before, after, ok := strings.Cut(rest, "("); ok {
			modelName = before
			// Извлекаем fk=... из скобок
			after := after
			if before, _, ok := strings.Cut(after, ")"); ok {
				opts := before
				for opt := range strings.SplitSeq(opts, ",") {
					opt = strings.TrimSpace(opt)
					if after0, ok := strings.CutPrefix(opt, "fk="); ok {
						fkField = after0
					}
				}
			}
		}
		if fkField == "" {
			fkField = modelName + "ID" // конвенция по умолчанию
		}
		return RelationInfo{
			Type:    "BelongsTo",
			Model:   modelName,
			FKField: fkField,
		}
	}
	return RelationInfo{}
}

// genRelationMethods генерирует код методов загрузки связей для модели.
func genRelationMethods(model ModelInfo, allModels map[string]ModelInfo) ([]string, error) {
	var methods []string
	for _, rel := range model.Relations {
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

// genHasManyMethod генерирует метод Load<Relation> для загрузки слайса связанных объектов.
func genHasManyMethod(parent ModelInfo, rel RelationInfo, target ModelInfo) (string, error) {
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
		"FieldType":    "[]" + rel.Model, // тип поля в родителе
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

// genBelongsToMethod генерирует метод Load<Relation> для загрузки одиночного связанного объекта.
func genBelongsToMethod(parent ModelInfo, rel RelationInfo, target ModelInfo) (string, error) {
	// scanFields для целевой модели
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

// ----------------- FTS генераторы -----------------

func genSearchMethod(m ModelInfo) (string, error) {
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

func genSearchWithHeadlineMethod(m ModelInfo) (string, error) {
	var selectParts, scanParts []string
	// Для структуры SearchResult нам нужны поля снова
	type fieldStruct struct{ Name, Type string }
	var fieldsForStruct []fieldStruct

	for _, f := range m.Fields {
		selectParts = append(selectParts, f.ColumnName)
		scanParts = append(scanParts, "&item."+f.Name)
		fieldsForStruct = append(fieldsForStruct, fieldStruct{f.Name, f.Type})
	}

	// Используем первое FTS поле в ts_headline (можно улучшить)
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

// ExtractModelsFromDir сканирует все Go-файлы в директории и возвращает найденные модели.
func ExtractModelsFromDir(dir string) ([]ModelInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("в пакете %s есть ошибки компиляции", dir)
	}

	var allModels []ModelInfo
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
	return allModels, nil
}

// Выполнено с любовью для Босса 🐈
