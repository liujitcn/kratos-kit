package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeClient struct {
	output *secretsmanager.GetSecretValueOutput
	input  *secretsmanager.GetSecretValueInput
}

func (f *fakeClient) GetSecretValue(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.input = input
	return f.output, nil
}

func TestProviderGet(t *testing.T) {
	version := "version-1"
	client := &fakeClient{output: &secretsmanager.GetSecretValueOutput{SecretString: stringPtr("root"), VersionId: &version}}
	provider := newProvider(client, WithVersionStage("AWSCURRENT"))
	secret, err := provider.Get(context.Background(), "prod/root", "version-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(secret.Value) != "root" || secret.Version != version {
		t.Fatalf("unexpected secret: %+v", secret)
	}
	if *client.input.VersionId != version || *client.input.VersionStage != "AWSCURRENT" {
		t.Fatalf("unexpected input: %+v", client.input)
	}
}

func stringPtr(value string) *string {
	return &value
}
