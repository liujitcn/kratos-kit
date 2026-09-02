package key

// Type 表示密钥 Provider 类型。
type Type string

const (
	// Local 表示本地文件密钥 Provider，也是默认 Provider。
	Local Type = "file"
	// Vault 表示 HashiCorp Vault 密钥 Provider。
	Vault Type = "vault"
	// AWS 表示 AWS Secrets Manager 密钥 Provider。
	AWS Type = "aws"
	// Google 表示 Google Secret Manager 密钥 Provider。
	Google Type = "google"
	// Azure 表示 Azure Key Vault 密钥 Provider。
	Azure Type = "azure"
	// Kubernetes 表示 Kubernetes Secret 密钥 Provider。
	Kubernetes Type = "kubernetes"
)
