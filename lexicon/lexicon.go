package lexicon
import (
	"context"	
	"fmt"		
	"io/fs"		
	"os"		
	"path/filepath"	
	"strings"	
	"sync"		
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/schema"	
	"gopkg.in/yaml.v3"			
)
type SchemaStatus string
const (
	SchemaStatusDraft	SchemaStatus	= "draft"	
	SchemaStatusActive	SchemaStatus	= "active"	
	SchemaStatusDeprecated	SchemaStatus	= "deprecated"	
	SchemaStatusArchived	SchemaStatus	= "archived"	
)
type LexiconDefinition struct {
	ID		string		`yaml:"id"`		
	Version		string		`yaml:"version"`	
	Name		string		`yaml:"name"`		
	Description	string		`yaml:"description"`	
	Status		SchemaStatus	`yaml:"status"`		
	Schema		string		`yaml:"schema"`		
}
type Registry struct {
	mu		sync.RWMutex			
	definitions	map[string]*LexiconDefinition	
	compiledTypes	map[string]*schema.TypeSystem	
	schemasDir	string				
}
func NewRegistry(schemasDir string) *Registry {
	return &Registry{
		definitions:	make(map[string]*LexiconDefinition),
		compiledTypes:	make(map[string]*schema.TypeSystem),
		schemasDir:	schemasDir,
	}
}
func (r *Registry) LoadSchemas(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return filepath.WalkDir(r.schemasDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", path, err)
		}
		var def LexiconDefinition
		if err := yaml.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("failed to parse schema file %s: %w", path, err)
		}
		if err := r.validateDefinition(&def); err != nil {
			return fmt.Errorf("invalid schema in %s: %w", path, err)
		}
		r.definitions[def.ID] = &def
		return nil
	})
}
func (r *Registry) GetSchema(id string) (*LexiconDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, exists := r.definitions[id]
	if !exists {
		return nil, fmt.Errorf("schema not found: %s", id)
	}
	return def, nil
}
func (r *Registry) GetCompiledSchema(id string) (*schema.TypeSystem, error) {
	r.mu.RLock()
	compiled, exists := r.compiledTypes[id]
	r.mu.RUnlock()
	if exists {
		return compiled, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if compiled, exists := r.compiledTypes[id]; exists {
		return compiled, nil
	}
	def, exists := r.definitions[id]
	if !exists {
		return nil, fmt.Errorf("schema not found: %s", id)
	}
	compiled, err := r.compileSchema(def.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema %s: %w", id, err)
	}
	r.compiledTypes[id] = compiled
	return compiled, nil
}
func (r *Registry) ValidateData(id string, data interface{}) error {
	compiled, err := r.GetCompiledSchema(id)
	if err != nil {
		return err
	}
	var rootType schema.Type
	for _, typ := range compiled.GetTypes() {
		rootType = typ
		break
	}
	if rootType == nil {
		return fmt.Errorf("no types found in schema %s", id)
	}
	return r.validateAgainstType(rootType, data)
}
func (r *Registry) ListSchemas() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]string, 0, len(r.definitions))
	for id := range r.definitions {
		schemas = append(schemas, id)
	}
	return schemas
}
func (r *Registry) ReloadSchemas(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions = make(map[string]*LexiconDefinition)
	r.compiledTypes = make(map[string]*schema.TypeSystem)
	return r.LoadSchemas(ctx)
}
func (r *Registry) validateDefinition(def *LexiconDefinition) error {
	if def.ID == "" {
		return fmt.Errorf("schema ID cannot be empty")
	}
	if def.Version == "" {
		return fmt.Errorf("schema version cannot be empty")
	}
	if def.Schema == "" {
		return fmt.Errorf("schema definition cannot be empty")
	}
	if def.Status != "active" && def.Status != "draft" && def.Status != "deprecated" {
		return fmt.Errorf("invalid status: %s", def.Status)
	}
	_, err := r.compileSchema(def.Schema)
	if err != nil {
		return fmt.Errorf("schema compilation failed: %w", err)
	}
	return nil
}
func (r *Registry) compileSchema(schemaText string) (*schema.TypeSystem, error) {
	typeSystem, err := ipld.LoadSchemaBytes([]byte(schemaText))
	if err != nil {
		return nil, fmt.Errorf("failed to load and compile schema: %w", err)
	}
	if typeSystem == nil {
		return nil, fmt.Errorf("compilation resulted in empty type system")
	}
	hasTypes := false
	for range typeSystem.GetTypes() {
		hasTypes = true
		break
	}
	if !hasTypes {
		return nil, fmt.Errorf("schema contains no valid types")
	}
	return typeSystem, nil
}
func (r *Registry) validateAgainstType(typ schema.Type, data interface{}) error {
	switch typ.TypeKind() {
	case schema.TypeKind_Struct:
		return r.validateStruct(typ, data)
	case schema.TypeKind_String:
		if _, ok := data.(string); !ok {
			return fmt.Errorf("expected string, got %T", data)
		}
	case schema.TypeKind_Bool:
		if _, ok := data.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", data)
		}
	case schema.TypeKind_Int:
		switch data.(type) {
		case int, int8, int16, int32, int64:
		default:
			return fmt.Errorf("expected int, got %T", data)
		}
	case schema.TypeKind_Float:
		switch data.(type) {
		case float32, float64:
		default:
			return fmt.Errorf("expected float, got %T", data)
		}
	case schema.TypeKind_List:
		return r.validateList(typ, data)
	case schema.TypeKind_Map:
		return r.validateMap(typ, data)
	}
	return nil
}
func (r *Registry) validateStruct(typ schema.Type, data interface{}) error {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map[string]interface{}, got %T", data)
	}
	structType, ok := typ.(*schema.TypeStruct)
	if !ok {
		return fmt.Errorf("expected *schema.TypeStruct, got %T", typ)
	}
	fields := structType.Fields()
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		fieldName := field.Name()
		value, exists := dataMap[fieldName]
		if !exists && !field.IsOptional() {
			return fmt.Errorf("required field missing: %s", fieldName)
		}
		if exists {
			if err := r.validateAgainstType(field.Type(), value); err != nil {
				return fmt.Errorf("field %s: %w", fieldName, err)
			}
		}
	}
	return nil
}
func (r *Registry) validateList(typ schema.Type, data interface{}) error {
	slice, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("expected []interface{}, got %T", data)
	}
	listType, ok := typ.(*schema.TypeList)
	if !ok {
		return fmt.Errorf("expected *schema.TypeList, got %T", typ)
	}
	valueType := listType.ValueType()
	for i, item := range slice {
		if err := r.validateAgainstType(valueType, item); err != nil {
			return fmt.Errorf("list item %d: %w", i, err)
		}
	}
	return nil
}
func (r *Registry) validateMap(typ schema.Type, data interface{}) error {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map[string]interface{}, got %T", data)
	}
	mapType, ok := typ.(*schema.TypeMap)
	if !ok {
		return fmt.Errorf("expected *schema.TypeMap, got %T", typ)
	}
	valueType := mapType.ValueType()
	for key, value := range dataMap {
		if err := r.validateAgainstType(valueType, value); err != nil {
			return fmt.Errorf("map key %s: %w", key, err)
		}
	}
	return nil
}
func (r *Registry) IsActive(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, exists := r.definitions[id]
	if !exists {
		return false
	}
	return def.Status == "active"
}