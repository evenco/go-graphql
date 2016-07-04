package graphql

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"

	"golang.org/x/net/context"

	"github.com/evenco/go-graphql/gqlerrors"
	"github.com/evenco/go-graphql/language/ast"
)

// These are all of the possible kinds of
type Type interface {
	GetName() string
	GetDescription() string
	String() string
	GetError() error
}

// <Even>

type TypeList []Type

func (l TypeList) Len() int {
	return len(l)
}

func (l TypeList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l TypeList) Less(i, j int) bool {
	return l[i].GetName() < l[j].GetName()
}

var _ sort.Interface = TypeList{}

// </Even>

var _ Type = (*Scalar)(nil)
var _ Type = (*Object)(nil)
var _ Type = (*Interface)(nil)
var _ Type = (*Union)(nil)
var _ Type = (*Enum)(nil)
var _ Type = (*InputObject)(nil)
var _ Type = (*List)(nil)
var _ Type = (*NonNull)(nil)
var _ Type = (*Argument)(nil)

// These types may be used as input types for arguments and directives.
type Input interface {
	GetName() string
	GetDescription() string
	String() string
	GetError() error
}

var _ Input = (*Scalar)(nil)
var _ Input = (*Enum)(nil)
var _ Input = (*InputObject)(nil)
var _ Input = (*List)(nil)
var _ Input = (*NonNull)(nil)

func IsInputType(ttype Type) bool {
	Named := GetNamed(ttype)
	if _, ok := Named.(*Scalar); ok {
		return true
	}
	if _, ok := Named.(*Enum); ok {
		return true
	}
	if _, ok := Named.(*InputObject); ok {
		return true
	}
	return false
}

func IsOutputType(ttype Type) bool {
	Named := GetNamed(ttype)
	if _, ok := Named.(*Scalar); ok {
		return true
	}
	if _, ok := Named.(*Object); ok {
		return true
	}
	if _, ok := Named.(*Interface); ok {
		return true
	}
	if _, ok := Named.(*Union); ok {
		return true
	}
	if _, ok := Named.(*Enum); ok {
		return true
	}
	return false
}

// These types may be used as output types as the result of fields.
type Output interface {
	GetName() string
	GetDescription() string
	String() string
	GetError() error
}

var _ Output = (*Scalar)(nil)
var _ Output = (*Object)(nil)
var _ Output = (*Interface)(nil)
var _ Output = (*Union)(nil)
var _ Output = (*Enum)(nil)
var _ Output = (*List)(nil)
var _ Output = (*NonNull)(nil)

// These types may describe the parent context of a selection set.
type Composite interface {
	GetName() string
}

var _ Composite = (*Object)(nil)
var _ Composite = (*Interface)(nil)
var _ Composite = (*Union)(nil)

// These types may describe the parent context of a selection set.
type Abstract interface {
	GetObjectType(value interface{}, info ResolveInfo) *Object
	GetPossibleTypes() []*Object
	IsPossibleType(ttype *Object) bool
}

var _ Abstract = (*Interface)(nil)
var _ Abstract = (*Union)(nil)

// These named types do not include modifiers like List or NonNull.
type Named interface {
	String() string
}

var _ Named = (*Scalar)(nil)
var _ Named = (*Object)(nil)
var _ Named = (*Interface)(nil)
var _ Named = (*Union)(nil)
var _ Named = (*Enum)(nil)
var _ Named = (*InputObject)(nil)

func GetNamed(ttype Type) Named {
	unmodifiedType := ttype
	for {
		if ttype, ok := unmodifiedType.(*List); ok {
			unmodifiedType = ttype.OfType
			continue
		}
		if ttype, ok := unmodifiedType.(*NonNull); ok {
			unmodifiedType = ttype.OfType
			continue
		}
		break
	}
	return unmodifiedType
}

/**
 * Scalar Type Definition
 *
 * The leaf values of any request and input values to arguments are
 * Scalars (or Enums) and are defined with a name and a series of functions
 * used to parse input from ast or variables and to ensure validity.
 *
 * Example:
 *
 *     var OddType = new Scalar({
 *       name: 'Odd',
 *       serialize(value) {
 *         return value % 2 === 1 ? value : null;
 *       }
 *     });
 *
 */
type Scalar struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	scalarConfig ScalarConfig
	err          error
}
type SerializeFn func(value interface{}) interface{}
type ParseValueFn func(value interface{}) interface{}
type ParseLiteralFn func(valueAST ast.Value) interface{}
type ScalarConfig struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Serialize    SerializeFn
	ParseValue   ParseValueFn
	ParseLiteral ParseLiteralFn
}

func NewScalar(config ScalarConfig) *Scalar {
	st := &Scalar{}
	if config.Name == "" {
		st.err = gqlerrors.NewFormattedError(context.Background(), "Type must be named.")
		return st
	}

	if err := assertValidName(config.Name); err != nil {
		st.err = err
		return st
	}

	st.Name = config.Name
	st.Description = config.Description

	if config.Serialize == nil {
		st.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v must provide "serialize" function. If this custom Scalar is `+
			`also used as an input type, ensure "parseValue" and "parseLiteral" `+
			`functions are also provided.`, st))
		return st
	}

	if config.ParseValue == nil || config.ParseLiteral == nil {
		st.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v must provide both "parseValue" and "parseLiteral" functions.`, st))
		return st
	}

	st.scalarConfig = config
	return st
}
func (st *Scalar) Serialize(value interface{}) interface{} {
	if st.scalarConfig.Serialize == nil {
		return value
	}
	return st.scalarConfig.Serialize(value)
}
func (st *Scalar) ParseValue(value interface{}) interface{} {
	if st.scalarConfig.ParseValue == nil {
		return value
	}
	return st.scalarConfig.ParseValue(value)
}
func (st *Scalar) ParseLiteral(valueAST ast.Value) interface{} {
	if st.scalarConfig.ParseLiteral == nil {
		return nil
	}
	return st.scalarConfig.ParseLiteral(valueAST)
}
func (st *Scalar) GetName() string {
	return st.Name
}
func (st *Scalar) GetDescription() string {
	return st.Description

}
func (st *Scalar) String() string {
	return st.Name
}
func (st *Scalar) GetError() error {
	return st.err
}

/**
 * Object Type Definition
 *
 * Almost all of the GraphQL types you define will be object  Object types
 * have a name, but most importantly describe their fields.
 *
 * Example:
 *
 *     var AddressType = new Object({
 *       name: 'Address',
 *       fields: {
 *         street: { type: String },
 *         number: { type: Int },
 *         formatted: {
 *           type: String,
 *           resolve(obj) {
 *             return obj.number + ' ' + obj.street
 *           }
 *         }
 *       }
 *     });
 *
 * When two types need to refer to each other, or a type needs to refer to
 * itself in a field, you can use a function expression (aka a closure or a
 * thunk) to supply the fields lazily.
 *
 * Example:
 *
 *     var PersonType = new Object({
 *       name: 'Person',
 *       fields: () => ({
 *         name: { type: String },
 *         bestFriend: { type: PersonType },
 *       })
 *     });
 *
 */
type Object struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsTypeOf    IsTypeOfFn

	typeConfig ObjectConfig
	fields     FieldDefinitionMap
	interfaces []*Interface
	// Interim alternative to throwing an error during schema definition at run-time
	err error
}

type IsTypeOfFn func(value interface{}, info ResolveInfo) bool

type InterfacesThunk func() []*Interface

type ObjectConfig struct {
	Name        string         `json:"description"`
	Interfaces  interface{}    `json:"interfaces"`
	Fields      FieldConfigMap `json:"fields"`
	IsTypeOf    IsTypeOfFn     `json:"isTypeOf"`
	Description string         `json:"description"`
}

func NewObject(config ObjectConfig) *Object {
	objectType := &Object{}

	if config.Name == "" {
		objectType.err = gqlerrors.NewFormattedError(context.Background(), "Type must be named.")
		return objectType
	}

	if err := assertValidName(config.Name); err != nil {
		objectType.err = err
		return objectType
	}

	objectType.Name = config.Name
	objectType.Description = config.Description
	objectType.IsTypeOf = config.IsTypeOf
	objectType.typeConfig = config

	/*
			addImplementationToInterfaces()
			Update the interfaces to know about this implementation.
			This is an rare and unfortunate use of mutation in the type definition
		 	implementations, but avoids an expensive "getPossibleTypes"
		 	implementation for Interface
	*/
	interfaces := objectType.GetInterfaces()
	if interfaces == nil {
		return objectType
	}
	for _, iface := range interfaces {
		iface.implementations = append(iface.implementations, objectType)
	}

	return objectType
}
func (gt *Object) AddFieldConfig(fieldName string, fieldConfig *FieldConfig) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	gt.typeConfig.Fields[fieldName] = fieldConfig

}
func (gt *Object) GetName() string {
	return gt.Name
}
func (gt *Object) GetDescription() string {
	return ""
}
func (gt *Object) String() string {
	return gt.Name
}
func (gt *Object) GetFields() FieldDefinitionMap {
	fields, err := defineFieldMap(gt, gt.typeConfig.Fields)
	gt.err = err
	gt.fields = fields
	return gt.fields
}
func (gt *Object) GetInterfaces() []*Interface {
	var configInterfaces []*Interface
	switch gt.typeConfig.Interfaces.(type) {
	case InterfacesThunk:
		configInterfaces = gt.typeConfig.Interfaces.(InterfacesThunk)()
	case []*Interface:
		configInterfaces = gt.typeConfig.Interfaces.([]*Interface)
	case nil:
	default:
		gt.err = errors.New(fmt.Sprintf("Unknown Object.Interfaces type: %v", reflect.TypeOf(gt.typeConfig.Interfaces)))
		return nil
	}
	interfaces, err := defineInterfaces(gt, configInterfaces)
	gt.err = err
	gt.interfaces = interfaces
	return gt.interfaces
}
func (gt *Object) GetError() error {
	return gt.err
}

func defineInterfaces(ttype *Object, interfaces []*Interface) ([]*Interface, error) {
	ifaces := []*Interface{}

	if len(interfaces) == 0 {
		return ifaces, nil
	}
	for _, iface := range interfaces {
		if iface == nil {
			return ifaces, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v may only implement Interface types, it cannot implement: %v.`, ttype, iface))
		}
		if iface.ResolveType == nil {
			return ifaces, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`Interface Type %v does not provide a "resolveType" function `+
				`and implementing Type %v does not provide a "isTypeOf" `+
				`function. There is no way to resolve this implementing type `+
				`during execution.`, iface, ttype))
		}
		ifaces = append(ifaces, iface)
	}

	return ifaces, nil
}

func defineFieldMap(ttype Named, fields FieldConfigMap) (FieldDefinitionMap, error) {

	resultFieldMap := FieldDefinitionMap{}

	if len(fields) == 0 {
		return resultFieldMap, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v fields must be an object with field names as keys or a function which return such an object.`, ttype))
	}

	for fieldName, field := range fields {
		if field == nil {
			continue
		}
		if field.Type == nil {
			return resultFieldMap, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v.%v field type must be Output Type but got: %v.`, ttype, fieldName, field.Type))
		}
		if field.Type.GetError() != nil {
			return resultFieldMap, field.Type.GetError()
		}
		if err := assertValidName(fieldName); err != nil {
			return resultFieldMap, err
		}
		fieldDef := &FieldDefinition{
			Name:              fieldName,
			Description:       field.Description,
			Type:              field.Type,
			Resolve:           field.Resolve,
			DeprecationReason: field.DeprecationReason,
		}

		fieldDef.Args = []*Argument{}
		for argName, arg := range field.Args {
			if err := assertValidName(argName); err != nil {
				return resultFieldMap, err
			}
			if arg == nil {
				return resultFieldMap, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v.%v args must be an object with argument names as keys.`, ttype, fieldName))
			}
			if arg.Type == nil {
				return resultFieldMap, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v.%v(%v:) argument type must be Input Type but got: %v.`, ttype, fieldName, argName, arg.Type))
			}
			fieldArg := &Argument{
				Name:         argName,
				Description:  arg.Description,
				Type:         arg.Type,
				DefaultValue: arg.DefaultValue,
			}
			fieldDef.Args = append(fieldDef.Args, fieldArg)
		}

		// <Even />

		argList := ArgumentList(fieldDef.Args)
		sort.Sort(argList)
		fieldDef.Args = []*Argument(argList)

		// </Even>

		resultFieldMap[fieldName] = fieldDef
	}
	return resultFieldMap, nil
}

// TODO: clean up GQLFRParams fields
type GQLFRParams struct {
	Source interface{}
	Args   map[string]interface{}
	Info   ResolveInfo
	Schema Schema
}

// TODO: relook at FieldResolveFn params
type FieldResolveFn func(ctx context.Context, p GQLFRParams) interface{}

type ResolveInfo struct {
	FieldName      string
	FieldASTs      []*ast.Field
	ReturnType     Output
	ParentType     Composite
	Schema         Schema
	Fragments      map[string]ast.Definition
	RootValue      interface{}
	Operation      ast.Definition
	VariableValues map[string]interface{}
}

type FieldConfigMap map[string]*FieldConfig

type FieldConfig struct {
	Name              string              `json:"name"` // used by graphlql-relay
	Type              Output              `json:"type"`
	Args              FieldConfigArgument `json:"args"`
	Resolve           FieldResolveFn
	DeprecationReason string `json:"deprecationReason"`
	Description       string `json:"description"`
}

type FieldConfigArgument map[string]*ArgumentConfig

type ArgumentConfig struct {
	Type         Input       `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

type FieldDefinitionMap map[string]*FieldDefinition
type FieldDefinition struct {
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Type              Output         `json:"type"`
	Args              []*Argument    `json:"args"`
	Resolve           FieldResolveFn `json:"-"`
	DeprecationReason string         `json:"deprecationReason"`
}

// <Even>

type FieldDefinitionList []*FieldDefinition

func (l FieldDefinitionList) Len() int {
	return len(l)
}

func (l FieldDefinitionList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l FieldDefinitionList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

var _ sort.Interface = FieldDefinitionList{}

// </Even>

type FieldArgument struct {
	Name         string      `json:"name"`
	Type         Type        `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

type Argument struct {
	Name         string      `json:"name"`
	Type         Input       `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

func (st *Argument) GetName() string {
	return st.Name
}
func (st *Argument) GetDescription() string {
	return st.Description

}
func (st *Argument) String() string {
	return st.Name
}
func (st *Argument) GetError() error {
	return nil
}

// <Even>

type ArgumentList []*Argument

func (l ArgumentList) Len() int {
	return len(l)
}

func (l ArgumentList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l ArgumentList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

var _ sort.Interface = ArgumentList{}

// </Even>

/**
 * Interface Type Definition
 *
 * When a field can return one of a heterogeneous set of types, a Interface type
 * is used to describe what types are possible, what fields are in common across
 * all types, as well as a function to determine which type is actually used
 * when the field is resolved.
 *
 * Example:
 *
 *     var EntityType = new Interface({
 *       name: 'Entity',
 *       fields: {
 *         name: { type: String }
 *       }
 *     });
 *
 */
type Interface struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ResolveType ResolveTypeFn

	typeConfig      InterfaceConfig
	fields          FieldDefinitionMap
	implementations []*Object
	possibleTypes   map[string]bool

	err error
}
type InterfaceConfig struct {
	Name        string         `json:"name"`
	Fields      FieldConfigMap `json:"fields"`
	ResolveType ResolveTypeFn
	Description string `json:"description"`
}
type ResolveTypeFn func(value interface{}, info ResolveInfo) *Object

func NewInterface(config InterfaceConfig) *Interface {
	it := &Interface{}
	if config.Name == "" {
		it.err = gqlerrors.NewFormattedError(context.Background(), "Type must be named.")
		return it
	}
	if err := assertValidName(config.Name); err != nil {
		it.err = err
		return it
	}
	it.Name = config.Name
	it.Description = config.Description
	it.ResolveType = config.ResolveType
	it.typeConfig = config
	it.implementations = []*Object{}
	return it
}

func (it *Interface) AddFieldConfig(fieldName string, fieldConfig *FieldConfig) {
	if fieldName == "" || fieldConfig == nil {
		return
	}
	it.typeConfig.Fields[fieldName] = fieldConfig
}
func (it *Interface) GetName() string {
	return it.Name
}
func (it *Interface) GetDescription() string {
	return it.Description
}
func (it *Interface) GetFields() (fields FieldDefinitionMap) {
	it.fields, it.err = defineFieldMap(it, it.typeConfig.Fields)
	return it.fields
}
func (it *Interface) GetPossibleTypes() []*Object {
	return it.implementations
}
func (it *Interface) IsPossibleType(ttype *Object) bool {
	if ttype == nil {
		return false
	}
	if len(it.possibleTypes) == 0 {
		possibleTypes := map[string]bool{}
		for _, possibleType := range it.GetPossibleTypes() {
			if possibleType == nil {
				continue
			}
			possibleTypes[possibleType.Name] = true
		}
		it.possibleTypes = possibleTypes
	}
	if val, ok := it.possibleTypes[ttype.Name]; ok {
		return val
	}
	return false
}
func (it *Interface) GetObjectType(value interface{}, info ResolveInfo) *Object {
	if it.ResolveType != nil {
		return it.ResolveType(value, info)
	}
	return getTypeOf(value, info, it)
}
func (it *Interface) String() string {
	return it.Name
}
func (it *Interface) GetError() error {
	return it.err
}

func getTypeOf(value interface{}, info ResolveInfo, abstractType Abstract) *Object {
	possibleTypes := abstractType.GetPossibleTypes()
	for _, possibleType := range possibleTypes {
		if possibleType.IsTypeOf == nil {
			continue
		}
		if res := possibleType.IsTypeOf(value, info); res {
			return possibleType
		}
	}
	return nil
}

/**
 * Union Type Definition
 *
 * When a field can return one of a heterogeneous set of types, a Union type
 * is used to describe what types are possible as well as providing a function
 * to determine which type is actually used when the field is resolved.
 *
 * Example:
 *
 *     var PetType = new Union({
 *       name: 'Pet',
 *       types: [ DogType, CatType ],
 *       resolveType(value) {
 *         if (value instanceof Dog) {
 *           return DogType;
 *         }
 *         if (value instanceof Cat) {
 *           return CatType;
 *         }
 *       }
 *     });
 *
 */
type Union struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ResolveType ResolveTypeFn

	typeConfig    UnionConfig
	types         []*Object
	possibleTypes map[string]bool

	err error
}
type UnionConfig struct {
	Name        string    `json:"name"`
	Types       []*Object `json:"types"`
	ResolveType ResolveTypeFn
	Description string `json:"description"`
}

func NewUnion(config UnionConfig) *Union {
	objectType := &Union{}

	if config.Name == "" {
		objectType.err = gqlerrors.NewFormattedError(context.Background(), "Type must be named.")
		return objectType
	}

	if err := assertValidName(config.Name); err != nil {
		objectType.err = err
		return objectType
	}

	objectType.Name = config.Name
	objectType.Description = config.Description
	objectType.ResolveType = config.ResolveType

	if len(config.Types) == 0 {
		objectType.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`Must provide Array of types for Union %v.`, config.Name))
		return objectType
	}

	for _, ttype := range config.Types {
		if ttype == nil {
			objectType.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v may only contain Object types, it cannot contain: %v.`, objectType, ttype))
			return objectType
		}
		if objectType.ResolveType == nil {
			if ttype.IsTypeOf == nil {
				objectType.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`Union Type %v does not provide a "resolveType" function `+
					`and possible Type %v does not provide a "isTypeOf" `+
					`function. There is no way to resolve this possible type `+
					`during execution.`, objectType, ttype))
				return objectType
			}
		}
	}
	objectType.types = config.Types
	objectType.typeConfig = config

	return objectType
}
func (ut *Union) GetPossibleTypes() []*Object {
	return ut.types
}
func (ut *Union) IsPossibleType(ttype *Object) bool {

	if ttype == nil {
		return false
	}
	if len(ut.possibleTypes) == 0 {
		possibleTypes := map[string]bool{}
		for _, possibleType := range ut.GetPossibleTypes() {
			if possibleType == nil {
				continue
			}
			possibleTypes[possibleType.Name] = true
		}
		ut.possibleTypes = possibleTypes
	}

	if val, ok := ut.possibleTypes[ttype.Name]; ok {
		return val
	}
	return false
}
func (ut *Union) GetObjectType(value interface{}, info ResolveInfo) *Object {
	if ut.ResolveType != nil {
		return ut.ResolveType(value, info)
	}
	return getTypeOf(value, info, ut)
}
func (ut *Union) String() string {
	return ut.Name
}
func (ut *Union) GetName() string {
	return ut.Name
}
func (ut *Union) GetDescription() string {
	return ut.Description
}
func (ut *Union) GetError() error {
	return ut.err
}

/**
 * Enum Type Definition
 *
 * Some leaf values of requests and input values are Enums. GraphQL serializes
 * Enum values as strings, however internally Enums can be represented by any
 * kind of type, often integers.
 *
 * Example:
 *
 *     var RGBType = new Enum({
 *       name: 'RGB',
 *       values: {
 *         RED: { value: 0 },
 *         GREEN: { value: 1 },
 *         BLUE: { value: 2 }
 *       }
 *     });
 *
 * Note: If a value is not provided in a definition, the name of the enum value
 * will be used as it's internal value.
 */
type Enum struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	enumConfig   EnumConfig
	values       []*EnumValueDefinition
	valuesLookup map[interface{}]*EnumValueDefinition
	nameLookup   map[string]*EnumValueDefinition

	err error
}
type EnumValueConfigMap map[string]*EnumValueConfig
type EnumValueConfig struct {
	Value             interface{} `json:"value"`
	DeprecationReason string      `json:"deprecationReason"`
	Description       string      `json:"description"`
}
type EnumConfig struct {
	Name        string             `json:"name"`
	Values      EnumValueConfigMap `json:"values"`
	Description string             `json:"description"`
}
type EnumValueDefinition struct {
	Name              string      `json:"name"`
	Value             interface{} `json:"value"`
	DeprecationReason string      `json:"deprecationReason"`
	Description       string      `json:"description"`
}

// <Even>

type EnumValueDefinitionList []*EnumValueDefinition

func (l EnumValueDefinitionList) Len() int {
	return len(l)
}

func (l EnumValueDefinitionList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l EnumValueDefinitionList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

var _ sort.Interface = EnumValueDefinitionList{}

// </Even>

func NewEnum(config EnumConfig) *Enum {
	gt := &Enum{}
	gt.enumConfig = config

	err := assertValidName(config.Name)
	if err != nil {
		gt.err = err
		return gt
	}

	gt.Name = config.Name
	gt.Description = config.Description
	gt.values, err = gt.defineEnumValues(config.Values)
	if err != nil {
		gt.err = err
		return gt
	}

	return gt
}
func (gt *Enum) defineEnumValues(valueMap EnumValueConfigMap) ([]*EnumValueDefinition, error) {
	values := []*EnumValueDefinition{}

	if len(valueMap) == 0 {
		return values, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v values must be an object with value names as keys.`, gt))
	}

	for valueName, valueConfig := range valueMap {
		if valueConfig == nil {
			return values, gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v.%v must refer to an object with a "value" key `+
				`representing an internal value but got: %v.`, gt, valueName, valueConfig))
		}
		if err := assertValidName(valueName); err != nil {
			return values, err
		}
		value := &EnumValueDefinition{
			Name:              valueName,
			Value:             valueConfig.Value,
			DeprecationReason: valueConfig.DeprecationReason,
			Description:       valueConfig.Description,
		}
		if value.Value == nil {
			value.Value = valueName
		}
		values = append(values, value)
	}

	// <Even>

	valuesList := EnumValueDefinitionList(values)
	sort.Sort(valuesList)
	values = []*EnumValueDefinition(valuesList)

	// </Even>

	return values, nil
}
func (gt *Enum) GetValues() []*EnumValueDefinition {
	return gt.values
}
func (gt *Enum) Serialize(value interface{}) interface{} {
	if enumValue, ok := gt.getValueLookup()[value]; ok {
		return enumValue.Name
	}
	return nil
}
func (gt *Enum) ParseValue(value interface{}) interface{} {
	valueStr, ok := value.(string)
	if !ok {
		return nil
	}
	if enumValue, ok := gt.getNameLookup()[valueStr]; ok {
		return enumValue.Value
	}
	return nil
}
func (gt *Enum) ParseLiteral(valueAST ast.Value) interface{} {
	if valueAST, ok := valueAST.(*ast.EnumValue); ok {
		if enumValue, ok := gt.getNameLookup()[valueAST.Value]; ok {
			return enumValue.Value
		}
	}
	return nil
}
func (gt *Enum) GetName() string {
	return gt.Name
}
func (gt *Enum) GetDescription() string {
	return ""
}
func (gt *Enum) String() string {
	return gt.Name
}
func (gt *Enum) GetError() error {
	return gt.err
}
func (gt *Enum) getValueLookup() map[interface{}]*EnumValueDefinition {
	if len(gt.valuesLookup) > 0 {
		return gt.valuesLookup
	}
	valuesLookup := map[interface{}]*EnumValueDefinition{}
	for _, value := range gt.GetValues() {
		valuesLookup[value.Value] = value
	}
	gt.valuesLookup = valuesLookup
	return gt.valuesLookup
}

func (gt *Enum) getNameLookup() map[string]*EnumValueDefinition {
	if len(gt.nameLookup) > 0 {
		return gt.nameLookup
	}
	nameLookup := map[string]*EnumValueDefinition{}
	for _, value := range gt.GetValues() {
		nameLookup[value.Name] = value
	}
	gt.nameLookup = nameLookup
	return gt.nameLookup
}

/**
 * Input Object Type Definition
 *
 * An input object defines a structured collection of fields which may be
 * supplied to a field argument.
 *
 * Using `NonNull` will ensure that a value must be provided by the query
 *
 * Example:
 *
 *     var GeoPoint = new InputObject({
 *       name: 'GeoPoint',
 *       fields: {
 *         lat: { type: new NonNull(Float) },
 *         lon: { type: new NonNull(Float) },
 *         alt: { type: Float, defaultValue: 0 },
 *       }
 *     });
 *
 */
type InputObject struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	typeConfig InputObjectConfig
	fields     InputObjectFieldMap

	err error
}
type InputObjectFieldConfig struct {
	Type         Input       `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}
type InputObjectField struct {
	Name         string      `json:"name"`
	Type         Input       `json:"type"`
	DefaultValue interface{} `json:"defaultValue"`
	Description  string      `json:"description"`
}

func (st *InputObjectField) GetName() string {
	return st.Name
}
func (st *InputObjectField) GetDescription() string {
	return st.Description

}
func (st *InputObjectField) String() string {
	return st.Name
}
func (st *InputObjectField) GetError() error {
	return nil
}

// <Even>

type InputObjectFieldList []*InputObjectField

func (l InputObjectFieldList) Len() int {
	return len(l)
}

func (l InputObjectFieldList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l InputObjectFieldList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

var _ sort.Interface = InputObjectFieldList{}

// </Even>

type InputObjectConfigFieldMap map[string]*InputObjectFieldConfig
type InputObjectFieldMap map[string]*InputObjectField
type InputObjectConfigFieldMapThunk func() InputObjectConfigFieldMap
type InputObjectConfig struct {
	Name        string      `json:"name"`
	Fields      interface{} `json:"fields"`
	Description string      `json:"description"`
}

// TODO: rename InputObjectConfig to GraphQLInputObjecTypeConfig for consistency?
func NewInputObject(config InputObjectConfig) *InputObject {
	gt := &InputObject{}

	if config.Name == "" {
		gt.err = gqlerrors.NewFormattedError(context.Background(), "Type must be named.")
		return gt
	}

	gt.Name = config.Name
	gt.Description = config.Description
	gt.typeConfig = config
	gt.fields = gt.defineFieldMap()
	return gt
}

func (gt *InputObject) defineFieldMap() InputObjectFieldMap {
	var fieldMap InputObjectConfigFieldMap
	switch gt.typeConfig.Fields.(type) {
	case InputObjectConfigFieldMap:
		fieldMap = gt.typeConfig.Fields.(InputObjectConfigFieldMap)
	case InputObjectConfigFieldMapThunk:
		fieldMap = gt.typeConfig.Fields.(InputObjectConfigFieldMapThunk)()
	}
	resultFieldMap := InputObjectFieldMap{}

	if len(fieldMap) == 0 {
		gt.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v fields must be an object with field names as keys or a function which return such an object.`, gt))
		return resultFieldMap
	}

	for fieldName, fieldConfig := range fieldMap {
		if fieldConfig == nil {
			continue
		}
		err := assertValidName(fieldName)
		if err != nil {
			continue
		}
		if fieldConfig.Type == nil {
			gt.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`%v.%v field type must be Input Type but got: %v.`, gt, fieldName, fieldConfig.Type))
			return resultFieldMap
		}
		field := &InputObjectField{}
		field.Name = fieldName
		field.Type = fieldConfig.Type
		field.Description = fieldConfig.Description
		field.DefaultValue = fieldConfig.DefaultValue
		resultFieldMap[fieldName] = field
	}
	return resultFieldMap
}
func (gt *InputObject) GetFields() InputObjectFieldMap {
	return gt.fields
}
func (gt *InputObject) GetName() string {
	return gt.Name
}
func (gt *InputObject) GetDescription() string {
	return gt.Description
}
func (gt *InputObject) String() string {
	return gt.Name
}
func (gt *InputObject) GetError() error {
	return gt.err
}

/**
 * List Modifier
 *
 * A list is a kind of type marker, a wrapping type which points to another
 * type. Lists are often created within the context of defining the fields of
 * an object type.
 *
 * Example:
 *
 *     var PersonType = new Object({
 *       name: 'Person',
 *       fields: () => ({
 *         parents: { type: new List(Person) },
 *         children: { type: new List(Person) },
 *       })
 *     })
 *
 */
type List struct {
	OfType Type `json:"ofType"`

	err error
}

func NewList(ofType Type) *List {
	gl := &List{}

	if ofType == nil {
		gl.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`Can only create List of a Type but got: %v.`, ofType))
		return gl
	}

	gl.OfType = ofType
	return gl
}
func (gl *List) GetName() string {
	return fmt.Sprintf("%v", gl.OfType)
}
func (gl *List) GetDescription() string {
	return ""
}
func (gl *List) String() string {
	if gl.OfType != nil {
		return fmt.Sprintf("[%v]", gl.OfType)
	}
	return ""
}
func (gl *List) GetError() error {
	return gl.err
}

/**
 * Non-Null Modifier
 *
 * A non-null is a kind of type marker, a wrapping type which points to another
 * type. Non-null types enforce that their values are never null and can ensure
 * an error is raised if this ever occurs during a request. It is useful for
 * fields which you can make a strong guarantee on non-nullability, for example
 * usually the id field of a database row will never be null.
 *
 * Example:
 *
 *     var RowType = new Object({
 *       name: 'Row',
 *       fields: () => ({
 *         id: { type: new NonNull(String) },
 *       })
 *     })
 *
 * Note: the enforcement of non-nullability occurs within the executor.
 */
type NonNull struct {
	Name   string `json:"name"` // added to conform with introspection for NonNull.Name = nil
	OfType Type   `json:"ofType"`

	err error
}

func NewNonNull(ofType Type) *NonNull {
	gl := &NonNull{}

	_, isOfTypeNonNull := ofType.(*NonNull)
	if ofType == nil || isOfTypeNonNull {
		gl.err = gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`Can only create NonNull of a Nullable Type but got: %v.`, ofType))
		return gl
	}
	gl.OfType = ofType
	return gl
}
func (gl *NonNull) GetName() string {
	return fmt.Sprintf("%v!", gl.OfType)
}
func (gl *NonNull) GetDescription() string {
	return ""
}
func (gl *NonNull) String() string {
	if gl.OfType != nil {
		return gl.GetName()
	}
	return ""
}
func (gl *NonNull) GetError() error {
	return gl.err
}

var NAME_REGEXP, _ = regexp.Compile("^[_a-zA-Z][_a-zA-Z0-9]*$")

func assertValidName(name string) error {
	if !NAME_REGEXP.MatchString(name) {
		return gqlerrors.NewFormattedError(context.Background(), fmt.Sprintf(`Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but "%v" does not.`, name))
	}
	return nil
}
