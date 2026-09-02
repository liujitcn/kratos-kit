package kubernetes

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestProviderGet(t *testing.T) {
	client := fake.NewSimpleClientset(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "default", ResourceVersion: "7"},
		Data:       map[string][]byte{"root-key": []byte("root")},
	})
	provider, err := NewFromClient(client, "default", WithValueKey("root-key"))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := provider.Get(context.Background(), "root", "")
	if err != nil {
		t.Fatal(err)
	}
	if string(secret.Value) != "root" || secret.Version != "7" {
		t.Fatalf("unexpected secret: %+v", secret)
	}
}
