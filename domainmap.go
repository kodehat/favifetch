package favifetch

// domainMappings maps package names and app identifiers to their canonical domains.
var domainMappings = map[string]string{
	"com.google.android.googlequicksearchbox": "google.com",
	"com.google.android.gm":                   "gmail.com",
	"com.pinterest":                           "pinterest.com",
	"snapchat":                                "snapchat.com",
}

// resolveDomainMapping returns the canonical domain if a mapping exists,
// or the original domain otherwise.
func resolveDomainMapping(domain string) string {
	if mapped, ok := domainMappings[domain]; ok {
		return mapped
	}
	return domain
}
