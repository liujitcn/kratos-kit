package redact

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Redactor 提供消息脱敏方法。
type Redactor interface {
	Redact()
}

// DynamicRedactor 提供可按请求上下文选择策略的消息脱敏方法。
type DynamicRedactor interface {
	RedactWith(ctx context.Context, resolver PolicyResolver)
}

type sceneContextKey struct{}

// WithScene 将脱敏场景写入上下文，解析器会优先匹配该场景的策略。
func WithScene(ctx context.Context, sceneCode string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if sceneCode == "" {
		sceneCode = "*"
	}
	return context.WithValue(ctx, sceneContextKey{}, sceneCode)
}

// SceneFromContext 从上下文读取脱敏场景，未设置时返回全局场景标识。
func SceneFromContext(ctx context.Context) string {
	if ctx == nil {
		return "*"
	}
	sceneCode, ok := ctx.Value(sceneContextKey{}).(string)
	if !ok || sceneCode == "" {
		return "*"
	}
	return sceneCode
}

// Apply 对实现 Redactor 的值执行脱敏；未实现该接口的值保持不变。
func Apply(in any) {
	if redactor, ok := in.(Redactor); ok {
		redactor.Redact()
	}
}

// PolicyMode 表示运行时策略对字段值的处理方式。
type PolicyMode uint8

const (
	// PolicyModeApplyRule 表示执行 Transform 规则。
	PolicyModeApplyRule PolicyMode = iota + 1
	// PolicyModeHide 表示将字段替换为对应类型的零值。
	PolicyModeHide
	// PolicyModeFull 表示保留字段原值。
	PolicyModeFull
)

// FieldPolicy 表示单个字段的运行时脱敏策略。
type FieldPolicy struct {
	Mode      PolicyMode
	Transform func(value any) any
}

// NewFieldPolicy 根据数据库中的策略模式、规则类型和 JSON 规则创建字段策略。
func NewFieldPolicy(mode PolicyMode, ruleType, ruleJSON string) (FieldPolicy, error) {
	switch mode {
	case PolicyModeHide, PolicyModeFull:
		return FieldPolicy{Mode: mode}, nil
	case PolicyModeApplyRule:
		transform, err := newRuleTransform(ruleType, ruleJSON)
		if err != nil {
			return FieldPolicy{}, err
		}
		return FieldPolicy{Mode: mode, Transform: transform}, nil
	default:
		return FieldPolicy{}, fmt.Errorf("不支持的脱敏策略模式: %d", mode)
	}
}

// Apply 将运行时策略应用到字段值。
func (p FieldPolicy) Apply(value any) any {
	switch p.Mode {
	case PolicyModeFull:
		return value
	case PolicyModeHide:
		return hiddenValue(value)
	case PolicyModeApplyRule:
		if p.Transform != nil {
			return p.Transform(value)
		}
	}
	return value
}

// PolicyResolver 按字段完整标识和上下文场景解析运行时策略。
type PolicyResolver interface {
	Resolve(ctx context.Context, fieldRef string) (FieldPolicy, bool)
}

var defaultPolicyResolver struct {
	sync.RWMutex
	value PolicyResolver
}

// SetDefaultPolicyResolver 设置进程级默认运行时策略解析器。
func SetDefaultPolicyResolver(resolver PolicyResolver) {
	defaultPolicyResolver.Lock()
	defaultPolicyResolver.value = resolver
	defaultPolicyResolver.Unlock()
}

// DefaultPolicyResolver 返回进程级默认运行时策略解析器。
func DefaultPolicyResolver() PolicyResolver {
	defaultPolicyResolver.RLock()
	resolver := defaultPolicyResolver.value
	defaultPolicyResolver.RUnlock()
	return resolver
}

// ApplyWith 按运行时策略执行动态脱敏；未配置解析器时执行消息默认规则。
func ApplyWith(ctx context.Context, resolver PolicyResolver, in any) {
	if resolver == nil {
		resolver = DefaultPolicyResolver()
	}
	if resolver == nil {
		Apply(in)
		return
	}
	if dynamic, ok := in.(DynamicRedactor); ok {
		dynamic.RedactWith(ctx, resolver)
		return
	}
	if message, ok := in.(proto.Message); ok {
		applyDynamicMessage(ctx, resolver, message.ProtoReflect())
		return
	}
	Apply(in)
}

// ApplyDynamic 尝试对单个字段应用运行时策略。
func ApplyDynamic(ctx context.Context, resolver PolicyResolver, fieldRef string, value any) (any, bool) {
	if resolver == nil {
		return value, false
	}
	policy, ok := resolver.Resolve(ctx, fieldRef)
	if !ok {
		return value, false
	}
	return policy.Apply(value), true
}

func applyDynamicMessage(ctx context.Context, resolver PolicyResolver, message protoreflect.Message) {
	if !message.IsValid() {
		return
	}
	fields := message.Descriptor().Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		fieldRef := string(message.Descriptor().FullName()) + "." + string(field.Name())
		if field.IsMap() {
			applyDynamicMap(ctx, resolver, fieldRef, message.Get(field).Map(), field.MapValue())
			continue
		}
		if field.IsList() {
			applyDynamicList(ctx, resolver, fieldRef, message.Get(field).List(), field)
			continue
		}
		if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
			if message.Has(field) {
				ApplyWith(ctx, resolver, message.Get(field).Message().Interface())
			}
			continue
		}
		if field.HasPresence() && !message.Has(field) {
			continue
		}
		value, applied := ApplyDynamic(ctx, resolver, fieldRef, message.Get(field).Interface())
		if applied {
			if converted, ok := dynamicReflectValue(field, value); ok {
				message.Set(field, converted)
			}
		}
	}
}

func applyDynamicList(ctx context.Context, resolver PolicyResolver, fieldRef string, list protoreflect.List, field protoreflect.FieldDescriptor) {
	for index := 0; index < list.Len(); index++ {
		value := list.Get(index)
		if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
			ApplyWith(ctx, resolver, value.Message().Interface())
			continue
		}
		transformed, applied := ApplyDynamic(ctx, resolver, fieldRef, value.Interface())
		if applied {
			if converted, ok := dynamicReflectValue(field, transformed); ok {
				list.Set(index, converted)
			}
		}
	}
}

func applyDynamicMap(ctx context.Context, resolver PolicyResolver, fieldRef string, values protoreflect.Map, field protoreflect.FieldDescriptor) {
	valueDescriptor := field.MapValue()
	values.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		if valueDescriptor.Kind() == protoreflect.MessageKind || valueDescriptor.Kind() == protoreflect.GroupKind {
			ApplyWith(ctx, resolver, value.Message().Interface())
			return true
		}
		transformed, applied := ApplyDynamic(ctx, resolver, fieldRef, value.Interface())
		if applied {
			if converted, ok := dynamicReflectValue(valueDescriptor, transformed); ok {
				values.Set(key, converted)
			}
		}
		return true
	})
}

func dynamicReflectValue(field protoreflect.FieldDescriptor, value any) (protoreflect.Value, bool) {
	switch field.Kind() {
	case protoreflect.BoolKind:
		converted, ok := value.(bool)
		return protoreflect.ValueOfBool(converted), ok
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		converted, ok := value.(int32)
		return protoreflect.ValueOfInt32(converted), ok
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		converted, ok := value.(int64)
		return protoreflect.ValueOfInt64(converted), ok
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		converted, ok := value.(uint32)
		return protoreflect.ValueOfUint32(converted), ok
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		converted, ok := value.(uint64)
		return protoreflect.ValueOfUint64(converted), ok
	case protoreflect.FloatKind:
		converted, ok := value.(float32)
		return protoreflect.ValueOfFloat32(converted), ok
	case protoreflect.DoubleKind:
		converted, ok := value.(float64)
		return protoreflect.ValueOfFloat64(converted), ok
	case protoreflect.StringKind:
		converted, ok := value.(string)
		return protoreflect.ValueOfString(converted), ok
	case protoreflect.BytesKind:
		converted, ok := value.([]byte)
		return protoreflect.ValueOfBytes(converted), ok
	case protoreflect.EnumKind:
		converted, ok := value.(protoreflect.EnumNumber)
		return protoreflect.ValueOfEnum(converted), ok
	default:
		return protoreflect.Value{}, false
	}
}

// Regex 根据正则表达式替换字符串内容。
func Regex(value, pattern, replacement string) string {
	compiledValue, ok := regexCache.Load(pattern)
	if !ok {
		var compiled *regexp.Regexp
		var err error
		compiled, err = regexp.Compile(pattern)
		if err != nil {
			return value
		}
		compiledValue, _ = regexCache.LoadOrStore(pattern, compiled)
	}
	return compiledValue.(*regexp.Regexp).ReplaceAllString(value, replacement)
}

// Mask 保留字符串首尾指定数量的字节，并用掩码字符替换中间内容。
func Mask(value string, keepFirst, keepLast int, maskChar string) string {
	if len(value) <= keepFirst+keepLast {
		return value
	}
	return value[:keepFirst] + strings.Repeat(maskChar, len(value)-keepFirst-keepLast) + value[len(value)-keepLast:]
}

// Email 按邮箱本地部分和域名分别执行掩码。
func Email(value string, keepLocalFirst int, maskDomain bool, maskChar string) string {
	at := strings.LastIndex(value, "@")
	if at < 0 {
		return value
	}
	local := value[:at]
	domain := value[at+1:]
	if len(local) > keepLocalFirst {
		local = local[:keepLocalFirst] + strings.Repeat(maskChar, len(local)-keepLocalFirst)
	}
	if maskDomain {
		domain = strings.Repeat(maskChar, len(domain))
	}
	return local + "@" + domain
}

// Truncate 保留字符串前指定长度的字节，并追加后缀。
func Truncate(value string, length int, suffix string) string {
	if len(value) <= length {
		return value
	}
	return value[:length] + suffix
}

// HashMD5 使用 MD5 生成十六进制摘要。
func HashMD5(value string) string {
	digest := md5.Sum([]byte(value))
	return fmt.Sprintf("%x", digest[:])
}

// HashSHA1 使用 SHA-1 生成十六进制摘要。
func HashSHA1(value string) string {
	digest := sha1.Sum([]byte(value))
	return fmt.Sprintf("%x", digest[:])
}

// HashSHA256 使用 SHA-256 生成十六进制摘要。
func HashSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:])
}

// UUID 根据字符串生成稳定的 UUID v5 格式值。
func UUID(value string) string {
	digest := sha1.Sum([]byte(value))
	digest[6] = (digest[6] & 0x0f) | 0x50
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

// IP 保留 IP 地址前指定数量的网段，其余部分替换为掩码字符。
func IP(value string, keepOctets int, maskChar string) string {
	ip := net.ParseIP(value)
	if ip == nil {
		return value
	}
	if ip.To4() != nil {
		parts := strings.Split(value, ".")
		for i := keepOctets; i < 4 && i < len(parts); i++ {
			parts[i] = maskChar
		}
		return strings.Join(parts, ".")
	}
	parts := strings.Split(ip.String(), ":")
	for i := keepOctets; i < len(parts); i++ {
		parts[i] = maskChar
	}
	return strings.Join(parts, ":")
}

// URL 对 URL 查询参数值执行掩码。
func URL(value string, maskQuery bool, maskChar string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return value
	}
	if maskQuery && parsed.RawQuery != "" {
		values, _ := url.ParseQuery(parsed.RawQuery)
		for key := range values {
			values.Set(key, strings.Repeat(maskChar, len(values.Get(key))))
		}
		parsed.RawQuery = values.Encode()
	}
	return parsed.String()
}

// FixedLength 使用指定字符替换为与原值等长的字符串。
func FixedLength(value, maskChar string) string {
	if maskChar == "" {
		maskChar = "X"
	}
	return strings.Repeat(maskChar, len(value))
}

// Condition 根据环境变量值判断是否执行条件规则。
func Condition(envVar, envVal string) bool {
	value := os.Getenv(envVar)
	if envVal == "" {
		return value != ""
	}
	return value == envVal
}

// Bypass 判断当前请求是否允许绕过内部方法保护。
type Bypass interface {
	CheckInternal(ctx context.Context) bool
}

// Wrapper 将函数适配为 Bypass。
type Wrapper func(ctx context.Context) bool

// CheckInternal 执行绕过判断函数。
func (w Wrapper) CheckInternal(ctx context.Context) bool { return w(ctx) }

// Falsy 表示始终不允许绕过内部方法保护。
var Falsy = Wrapper(func(_ context.Context) bool {
	return false
})

// CustomRedactor 表示用户注册的字符串脱敏函数。
type CustomRedactor func(string) string

var (
	customRedactors   = map[string]CustomRedactor{}
	customRedactorsMu sync.RWMutex
	regexCache        sync.Map
)

// RegisterCustomRedactor 注册命名脱敏函数。
func RegisterCustomRedactor(name string, redactor CustomRedactor) {
	customRedactorsMu.Lock()
	defer customRedactorsMu.Unlock()
	customRedactors[name] = redactor
}

// ApplyCustomRedactor 执行指定名称的自定义脱敏函数；未注册时原样返回。
func ApplyCustomRedactor(name, value string) string {
	customRedactorsMu.RLock()
	redactor, ok := customRedactors[name]
	customRedactorsMu.RUnlock()
	if ok {
		return redactor(value)
	}
	return value
}

func hiddenValue(value any) any {
	if value == nil {
		return nil
	}
	return reflect.Zero(reflect.TypeOf(value)).Interface()
}

// newRuleTransform 根据规则类型和 JSON 配置构造字段转换函数。
func newRuleTransform(ruleType, ruleJSON string) (func(any) any, error) {
	var err error
	ruleType = strings.ToUpper(strings.TrimSpace(ruleType))
	var rules map[string]json.RawMessage
	err = json.Unmarshal([]byte(ruleJSON), &rules)
	if err != nil {
		return nil, fmt.Errorf("脱敏规则 JSON 无效: %w", err)
	}
	rawRule, ok := rules[strings.ToLower(ruleType)]
	if !ok {
		for key, value := range rules {
			if strings.EqualFold(key, ruleType) {
				rawRule = value
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("脱敏规则 JSON 缺少 %s 配置", ruleType)
	}

	switch ruleType {
	case "MASK":
		var rule struct {
			KeepFirst uint32 `json:"keep_first"`
			KeepLast  uint32 `json:"keep_last"`
			MaskChar  string `json:"mask_char"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("MASK 规则无效: %w", err)
		}
		if rule.MaskChar == "" {
			rule.MaskChar = "*"
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return Mask(text, int(rule.KeepFirst), int(rule.KeepLast), rule.MaskChar)
		}, nil
	case "REGEX":
		var rule struct {
			Pattern     string `json:"pattern"`
			Replacement string `json:"replacement"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("REGEX 规则无效: %w", err)
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return Regex(text, rule.Pattern, rule.Replacement)
		}, nil
	case "EMAIL":
		var rule struct {
			KeepLocalFirst uint32 `json:"keep_local_first"`
			MaskDomain     bool   `json:"mask_domain"`
			MaskChar       string `json:"mask_char"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("EMAIL 规则无效: %w", err)
		}
		if rule.MaskChar == "" {
			rule.MaskChar = "*"
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return Email(text, int(rule.KeepLocalFirst), rule.MaskDomain, rule.MaskChar)
		}, nil
	case "TRUNCATE":
		var rule struct {
			Length uint32 `json:"length"`
			Suffix string `json:"suffix"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("TRUNCATE 规则无效: %w", err)
		}
		if rule.Suffix == "" {
			rule.Suffix = "..."
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return Truncate(text, int(rule.Length), rule.Suffix)
		}, nil
	case "HASH":
		var rule struct {
			Algo string `json:"algo"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("HASH 规则无效: %w", err)
		}
		var transform func(string) string
		switch strings.ToUpper(rule.Algo) {
		case "MD5":
			transform = HashMD5
		case "SHA1":
			transform = HashSHA1
		case "SHA256":
			transform = HashSHA256
		default:
			return nil, fmt.Errorf("不支持的 HASH 算法: %s", rule.Algo)
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return transform(text)
		}, nil
	case "UUID":
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return UUID(text)
		}, nil
	case "IP":
		var rule struct {
			KeepOctets uint32 `json:"keep_octets"`
			MaskChar   string `json:"mask_char"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("IP 规则无效: %w", err)
		}
		if rule.MaskChar == "" {
			rule.MaskChar = "x"
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return IP(text, int(rule.KeepOctets), rule.MaskChar)
		}, nil
	case "URL":
		var rule struct {
			MaskQuery bool   `json:"mask_query"`
			MaskChar  string `json:"mask_char"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("URL 规则无效: %w", err)
		}
		if rule.MaskChar == "" {
			rule.MaskChar = "*"
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return URL(text, rule.MaskQuery, rule.MaskChar)
		}, nil
	case "FIXED_LENGTH":
		var rule struct {
			Char string `json:"char"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("FIXED_LENGTH 规则无效: %w", err)
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return FixedLength(text, rule.Char)
		}, nil
	case "CUSTOM":
		var rule struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(rawRule, &rule)
		if err != nil {
			return nil, fmt.Errorf("CUSTOM 规则无效: %w", err)
		}
		return func(value any) any {
			text, ok := value.(string)
			if !ok {
				return value
			}
			return ApplyCustomRedactor(rule.Name, text)
		}, nil
	default:
		return nil, fmt.Errorf("不支持的脱敏规则类型: %s", ruleType)
	}
}
