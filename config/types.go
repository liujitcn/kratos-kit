package config

type Type string

const (
	// TypeApollo 表示 Apollo 远程配置类型。
	TypeApollo Type = "apollo"

	// TypeConsul 表示 Consul 远程配置类型。
	TypeConsul Type = "consul"

	// TypeEtcd 表示 Etcd 远程配置类型。
	TypeEtcd Type = "etcd"

	// TypeKubernetes 表示 Kubernetes 远程配置类型。
	TypeKubernetes Type = "kubernetes"

	// TypeNacos 表示 Nacos 远程配置类型。
	TypeNacos Type = "nacos"

	// TypePolaris 表示 Polaris 远程配置类型。
	TypePolaris Type = "polaris"
)
