package webguard

// AppLink is a server-owned, validated navigation destination.
type AppLink struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Origin string `json:"origin"`
}

func appName(id string) string {
	if id == "noverse" {
		return "Noverse"
	}
	return "Viderse"
}

// LinksFor returns only destinations the authenticated account can access.
// Without explicit links, the current app is the sole candidate; a relative
// origin works for standalone deployments without PUBLIC_URL.
func (c *Config) LinksFor(current string, allowed func(string) bool) []AppLink {
	candidates := []AppLink(nil)
	if c != nil {
		candidates = c.AppLinks
	}
	if len(candidates) == 0 {
		origin := ""
		if c != nil && c.PublicURL != nil {
			origin = c.PublicURL.Scheme + "://" + c.PublicURL.Host
		}
		candidates = []AppLink{{ID: current, Name: appName(current), Origin: origin}}
	}
	out := make([]AppLink, 0, len(candidates))
	for _, link := range candidates {
		if allowed(link.ID) {
			out = append(out, link)
		}
	}
	return out
}
