package casbin

const DefaultWildcardItem = "*"

const DefaultAuthorizedProjectsMatcher = "r.tenant == p.tenant && r.sub == p.sub && (keyMatch(r.dom, p.dom) || p.dom == '*')"

const DefaultTenant = "default"
