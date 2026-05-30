package clientip

import (
	"context"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// GeoInfo 地理位置信息.
type GeoInfo struct {
	// Country 国家代码 (ISO 3166-1 alpha-2)
	Country string

	// CountryName 国家名称
	CountryName string

	// City 城市名称
	City string

	// Region 省/州/地区
	Region string

	// Latitude 纬度
	Latitude float64

	// Longitude 经度
	Longitude float64

	// TimeZone 时区
	TimeZone string

	// PostalCode 邮政编码
	PostalCode string

	// ASN 自治系统编号
	ASN uint

	// ASOrg 自治系统组织名称
	ASOrg string
}

// GeoResolver 地理位置解析器接口.
type GeoResolver interface {
	// Lookup 查询 IP 地址的地理位置信息.
	Lookup(ip string) (*GeoInfo, error)

	// Close 关闭解析器，释放资源.
	Close() error
}

// MaxMindResolver MaxMind GeoIP2 数据库解析器.
//
// 支持 GeoLite2-City 和 GeoIP2-City 数据库.
// 下载地址: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
type MaxMindResolver struct {
	cityDB *geoip2.Reader
	asnDB  *geoip2.Reader
	lang   string
}

// MaxMindOption MaxMind 解析器配置选项.
type MaxMindOption func(*MaxMindResolver)

// WithMaxMindASN 添加 ASN 数据库支持.
//
// ASN 数据库提供自治系统信息ISP、组织等.
func WithMaxMindASN(asnDBPath string) MaxMindOption {
	return func(r *MaxMindResolver) {
		db, err := geoip2.Open(asnDBPath)
		if err == nil {
			r.asnDB = db
		}
	}
}

// WithMaxMindLang 设置返回的地名语言.
//
// 默认: "zh-CN"
// 常用语言: "en", "zh-CN", "ja", "ko", "de", "fr", "es", "pt-BR", "ru"
func WithMaxMindLang(lang string) MaxMindOption {
	return func(r *MaxMindResolver) {
		r.lang = lang
	}
}

// NewMaxMindResolver 创建 MaxMind GeoIP2 解析器.
//
// cityDBPath: GeoLite2-City.mmdb 或 GeoIP2-City.mmdb 文件路径
//
// 示例:
//
//	resolver, err := clientip.NewMaxMindResolver("/path/to/GeoLite2-City.mmdb")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resolver.Close()
//
//	geo, err := resolver.Lookup("8.8.8.8")
func NewMaxMindResolver(cityDBPath string, opts ...MaxMindOption) (*MaxMindResolver, error) {
	db, err := geoip2.Open(cityDBPath)
	if err != nil {
		return nil, err
	}

	r := &MaxMindResolver{
		cityDB: db,
		lang:   "zh-CN",
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// Lookup 查询 IP 地址的地理位置信息.
func (r *MaxMindResolver) Lookup(ipStr string) (*GeoInfo, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, &InvalidIPError{IP: ipStr}
	}

	city, err := r.cityDB.City(ip)
	if err != nil {
		return nil, err
	}

	geo := &GeoInfo{
		Country:    city.Country.IsoCode,
		Latitude:   city.Location.Latitude,
		Longitude:  city.Location.Longitude,
		TimeZone:   city.Location.TimeZone,
		PostalCode: city.Postal.Code,
	}

	// 获取本地化名称
	if name, ok := city.Country.Names[r.lang]; ok {
		geo.CountryName = name
	} else if name, ok := city.Country.Names["en"]; ok {
		geo.CountryName = name
	}

	if name, ok := city.City.Names[r.lang]; ok {
		geo.City = name
	} else if name, ok := city.City.Names["en"]; ok {
		geo.City = name
	}

	if len(city.Subdivisions) > 0 {
		sub := city.Subdivisions[0]
		if name, ok := sub.Names[r.lang]; ok {
			geo.Region = name
		} else if name, ok := sub.Names["en"]; ok {
			geo.Region = name
		}
	}

	// 查询 ASN 信息如果配置了 ASN 数据库
	if r.asnDB != nil {
		if asn, err := r.asnDB.ASN(ip); err == nil {
			geo.ASN = asn.AutonomousSystemNumber
			geo.ASOrg = asn.AutonomousSystemOrganization
		}
	}

	return geo, nil
}

// Close 关闭数据库连接.
func (r *MaxMindResolver) Close() error {
	var err error
	if r.cityDB != nil {
		err = r.cityDB.Close()
	}
	if r.asnDB != nil {
		if e := r.asnDB.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// InvalidIPError 无效 IP 地址错误.
type InvalidIPError struct {
	IP string
}

func (e *InvalidIPError) Error() string {
	return "clientip: invalid IP address: " + e.IP
}

// WithGeoInfo 将地理位置信息存入 context.
func WithGeoInfo(ctx context.Context, geo *GeoInfo) context.Context {
	return context.WithValue(ctx, geoContextKey, geo)
}

// GeoInfoFromContext 从 context 获取地理位置信息.
func GeoInfoFromContext(ctx context.Context) (*GeoInfo, bool) {
	geo, ok := ctx.Value(geoContextKey).(*GeoInfo)
	return geo, ok
}

// GetCountry 从 context 获取国家代码.
//
// 便捷方法，如果不存在返回空字符串.
func GetCountry(ctx context.Context) string {
	if geo, ok := GeoInfoFromContext(ctx); ok {
		return geo.Country
	}
	return ""
}

// GetCity 从 context 获取城市名称.
//
// 便捷方法，如果不存在返回空字符串.
func GetCity(ctx context.Context) string {
	if geo, ok := GeoInfoFromContext(ctx); ok {
		return geo.City
	}
	return ""
}
