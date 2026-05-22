package bossq

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ModelInfo хранит разобранную информацию о модели.
type ModelInfo struct {
	Package      string
	ModelName    string
	StoreName    string
	TableName    string
	Fields       []FieldInfo
	UniqueFields []FieldInfo
	PKField      string
	PKColumn     string
	Relations    []RelationInfo
	Imports      []string
	SourceFile   string
	FTSLanguage  string
	FTSFields    []FTSFieldInfo
}

type FTSFieldInfo struct {
	FieldName  string
	ColumnName string
	Weight     string
}

type RelationInfo struct {
	Name       string
	Type       string
	Model      string
	ForeignKey string
	References string
	FKField    string
}

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
	FTSWeight  string
	IsUnique   bool
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
			models := extractModelsFromFile(file)
			for j := range models {
				models[j].Package = pkg.Name
				models[j].SourceFile = pkg.GoFiles[i]
			}
			allModels = append(allModels, models...)
		}
	}
	return allModels, nil
}

// extractModelsFromFile проходит по AST-файлу и собирает все структуры с тегом bossq.
func extractModelsFromFile(file *ast.File) []ModelInfo {
	var models []ModelInfo

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

			model := ModelInfo{
				ModelName:   typeSpec.Name.Name,
				StoreName:   typeSpec.Name.Name + "Store",
				TableName:   localTable,
				FTSLanguage: localFTS,
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
					model.FTSFields = append(model.FTSFields, FTSFieldInfo{
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

// toSnakeCase обрабатывает CamelCase и аббревиатуры.
func toSnakeCase(s string) string {
	abbreviations := map[string]string{
		"ID":    "id",
		"URL":   "url",
		"URI":   "uri",
		"HTTP":  "http",
		"HTTPS": "https",
		"API":   "api",
		"JSON":  "json",
		"XML":   "xml",
		"SQL":   "sql",
	}
	if lower, ok := abbreviations[s]; ok {
		return lower
	}
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i+1 < len(s) && s[i+1] >= 'A' && s[i+1] <= 'Z' {
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

func (f *FieldInfo) ParseTag(tag string) {
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
		case part == "unique":
			f.IsUnique = true
		case strings.HasPrefix(part, "type="):
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
				ForeignKey: parts[1],
			}
		}
	}
	if strings.HasPrefix(value, "belongsTo:") {
		parts := strings.Split(value[len("belongsTo:"):], ".")
		if len(parts) == 2 {
			return RelationInfo{
				Type:    "BelongsTo",
				Model:   parts[0],
				FKField: parts[1],
			}
		}
	}
	return RelationInfo{}
}

// Выполнено с любовью для Босса 🐈‍