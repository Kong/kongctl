package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"time"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/optionalnullable"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/kong/kongctl/internal/util"
)

type IdentityPrincipalKeysAPI interface {
	IdentityPrincipalKeysReader
	CreatePrincipalKey(context.Context, kkOps.CreatePrincipalKeyRequest, ...kkOps.Option) (
		*kkOps.CreatePrincipalKeyResponse, error,
	)
	UpdatePrincipalKey(context.Context, kkOps.UpdatePrincipalKeyRequest, ...kkOps.Option) (
		*kkOps.UpdatePrincipalKeyResponse, error,
	)
	DeletePrincipalKey(context.Context, kkOps.DeletePrincipalKeyRequest, ...kkOps.Option) (
		*kkOps.DeletePrincipalKeyResponse, error,
	)
}

type IdentityGroupKeysAPI interface {
	IdentityGroupKeysReader
	CreateGroupKey(context.Context, kkOps.CreateGroupKeyRequest, ...kkOps.Option) (*kkOps.CreateGroupKeyResponse, error)
	UpdateGroupKey(context.Context, kkOps.UpdateGroupKeyRequest, ...kkOps.Option) (*kkOps.UpdateGroupKeyResponse, error)
	DeleteGroupKey(context.Context, kkOps.DeleteGroupKeyRequest, ...kkOps.Option) (*kkOps.DeleteGroupKeyResponse, error)
}

// IdentityAPIKeyClient keeps every operation under one explicit parent. Key
// names are not unique, and administrative verification is not runtime proof.
type IdentityAPIKeyClient struct {
	principal   IdentityPrincipalKeysAPI
	group       IdentityGroupKeysAPI
	directoryID string
	parentID    string
}

func NewPrincipalAPIKeyClient(api IdentityPrincipalKeysAPI, directoryID, principalID string) (
	*IdentityAPIKeyClient, error,
) {
	if api == nil || !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return nil, fmt.Errorf("principal API-key API and directory/principal UUIDs are required")
	}
	return &IdentityAPIKeyClient{principal: api, directoryID: directoryID, parentID: principalID}, nil
}

func NewGroupAPIKeyClient(api IdentityGroupKeysAPI, directoryID, groupID string) (*IdentityAPIKeyClient, error) {
	if api == nil || !util.IsValidUUID(directoryID) || !util.IsValidUUID(groupID) {
		return nil, fmt.Errorf("group API-key API and directory/group UUIDs are required")
	}
	return &IdentityAPIKeyClient{group: api, directoryID: directoryID, parentID: groupID}, nil
}

type IdentityAPIKeyCreate struct {
	Name      *string
	Secret    *string
	ExpiresAt optionalnullable.OptionalNullable[time.Time]
	ManagedBy map[string]string
}

type IdentityAPIKeyUpdate struct {
	Name      *string
	NameSet   bool
	ExpiresAt optionalnullable.OptionalNullable[time.Time]
	ManagedBy map[string]*string
}

func (c *IdentityAPIKeyClient) List(ctx context.Context, pageSize int64) ([]IdentityAPIKey, error) {
	if c.group != nil {
		return ListIdentityGroupAPIKeys(ctx, c.group, c.directoryID, c.parentID, pageSize)
	}
	return ListPrincipalAPIKeys(ctx, c.principal, c.directoryID, c.parentID, pageSize)
}

func (c *IdentityAPIKeyClient) Get(ctx context.Context, id string) (*IdentityAPIKey, error) {
	if !util.IsValidUUID(id) {
		return nil, fmt.Errorf("API-key ID must be a UUID")
	}
	var raw *http.Response
	if c.group != nil {
		response, err := c.group.GetGroupKey(ctx, kkOps.GetGroupKeyRequest{
			DirectoryID: c.directoryID, GroupID: c.parentID, APIKeyID: id,
		})
		if err != nil {
			return nil, identitySecretAPIError("get group API key", err)
		}
		if response != nil {
			raw = response.RawResponse
		}
	} else {
		response, err := c.principal.GetIdentityKey(ctx, kkOps.GetIdentityKeyRequest{
			DirectoryID: c.directoryID, PrincipalID: c.parentID, APIKeyID: id,
		})
		if err != nil {
			return nil, identitySecretAPIError("get principal API key", err)
		}
		if response != nil {
			raw = response.RawResponse
		}
	}
	return identityAPIKeyWriteObservation(raw, id)
}

func identityAPIKeyWriteObservation(response *http.Response, expectedID string) (*IdentityAPIKey, error) {
	var raw struct {
		ID        string          `json:"id"`
		Name      *string         `json:"name"`
		ExpiresAt json.RawMessage `json:"expires_at"`
		ManagedBy json.RawMessage `json:"managed_by"`
		CreatedAt time.Time       `json:"created_at"`
		UpdatedAt time.Time       `json:"updated_at"`
	}
	if err := decodeIdentityCredentialResponse(response, &raw); err != nil {
		return nil, err
	}
	if !util.IsValidUUID(raw.ID) || expectedID != "" && raw.ID != expectedID ||
		len(raw.ExpiresAt) == 0 || len(raw.ManagedBy) == 0 || raw.CreatedAt.IsZero() || raw.UpdatedAt.IsZero() {
		return nil, fmt.Errorf("missing or inconsistent API-key identity, expiration, management metadata or timestamps")
	}
	value := &IdentityAPIKey{ID: raw.ID, Name: raw.Name, CreatedAt: raw.CreatedAt, UpdatedAt: raw.UpdatedAt}
	if err := json.Unmarshal(raw.ExpiresAt, &value.ExpiresAt); err != nil {
		return nil, fmt.Errorf("invalid API-key expiration response")
	}
	if err := json.Unmarshal(raw.ManagedBy, &value.ManagedBy); err != nil || value.ManagedBy == nil {
		return nil, fmt.Errorf("invalid API-key management metadata response")
	}
	return value, nil
}

func sameIdentityAPIKey(a, b *IdentityAPIKey) bool {
	return a.ID == b.ID && sameIdentityKeyName(a.Name, b.Name) &&
		sameIdentityCredentialExpiration(a.ExpiresAt, b.ExpiresAt) &&
		maps.Equal(a.ManagedBy, b.ManagedBy) && a.CreatedAt.Equal(b.CreatedAt)
}

func sameIdentityKeyName(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (c *IdentityAPIKeyClient) validateCreate(options IdentityAPIKeyCreate) error {
	if options.Secret != nil {
		if c.group != nil && options.Name != nil {
			return fmt.Errorf("imported group API keys cannot set a name at creation; create first, then rename")
		}
		return ValidateImportedIdentityAPIKeySecret(*options.Secret)
	}
	return nil
}

// ValidateImportedIdentityAPIKeySecret never includes material in diagnostics.
func ValidateImportedIdentityAPIKeySecret(value string) error {
	length := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || length < 1 || length > 255 {
		return fmt.Errorf("API-key material must contain 1 to 255 UTF-8 characters")
	}
	return nil
}

func (c *IdentityAPIKeyClient) captureMetadata(local secretoutput.Metadata) secretoutput.Metadata {
	metadata := secretoutput.Metadata{
		ResourceType: local.ResourceType, ResourceRef: local.ResourceRef,
		Namespace: local.Namespace, ParentRef: local.ParentRef, DirectoryID: c.directoryID,
	}
	if c.group != nil {
		metadata.GroupID = c.parentID
	} else {
		metadata.PrincipalID = c.parentID
	}
	return metadata
}

// CreateAndCapture reserves private output before the single create attempt.
// Failure after issuance never retries creation or automatically deletes a key.
func (c *IdentityAPIKeyClient) CreateAndCapture(ctx context.Context,
	options IdentityAPIKeyCreate, outputPath string,
) (*IdentityAPIKey, error) {
	if err := c.validateCreate(options); err != nil {
		return nil, err
	}
	metadata := c.captureMetadata(secretoutput.Metadata{})
	output, err := secretoutput.Reserve(outputPath, metadata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Close() }()
	return c.CreateWithReservedCapture(ctx, options, output, metadata)
}

// CreateWithReservedCapture accepts a file reserved during whole-plan preflight.
// Parent IDs are bound from this client before issuance; local context never
// changes the request scope. Imported material may omit capture because the
// caller already possesses it. Unsupported input leaves the reservation unused.
func (c *IdentityAPIKeyClient) CreateWithReservedCapture(ctx context.Context,
	options IdentityAPIKeyCreate, output *secretoutput.File, local secretoutput.Metadata,
) (*IdentityAPIKey, error) {
	if err := c.validateCreate(options); err != nil {
		return nil, err
	}
	if output == nil && options.Secret == nil {
		return nil, fmt.Errorf("generated API-key creation requires a reserved private capture")
	}
	metadata := c.captureMetadata(local)
	outputPath := ""
	if output != nil {
		if err := output.RecordPending(metadata); err != nil {
			return nil, err
		}
		defer func() { _ = output.Close() }()
		outputPath = output.Path()
	}
	body := kkComps.CreateCreatePrincipalKeyBodyV1(kkComps.CreatePrincipalKeyBodyv1{
		Name: options.Name, ExpiresAt: options.ExpiresAt, ManagedBy: maps.Clone(options.ManagedBy),
	})
	if options.Secret != nil {
		body = kkComps.CreateCreatePrincipalKeyBodyImported(kkComps.CreatePrincipalKeyBodyImported{
			Name: options.Name, Secret: *options.Secret, ExpiresAt: options.ExpiresAt, ManagedBy: maps.Clone(options.ManagedBy),
		})
	}
	var issued *kkComps.KongPrincipalKeyWithSecret
	var raw *http.Response
	var err error
	status := 0
	ctx = httpclient.WithoutRetries(ctx)
	if c.group != nil {
		response, requestErr := c.group.CreateGroupKey(ctx, kkOps.CreateGroupKeyRequest{
			DirectoryID: c.directoryID, GroupID: c.parentID, CreatePrincipalKeyBody: body,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
		err = requestErr
		if response != nil {
			issued, raw, status = response.KongPrincipalKeyWithSecret, response.RawResponse, response.StatusCode
		}
	} else {
		response, requestErr := c.principal.CreatePrincipalKey(ctx, kkOps.CreatePrincipalKeyRequest{
			DirectoryID: c.directoryID, PrincipalID: c.parentID, CreatePrincipalKeyBody: body,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
		err = requestErr
		if response != nil {
			issued, raw, status = response.KongPrincipalKeyWithSecret, response.RawResponse, response.StatusCode
		}
	}
	if err != nil {
		if output == nil {
			return nil, fmt.Errorf("%w; creation outcome is unconfirmed; inspect parent metadata before retrying",
				identitySecretAPIError("create API key", err))
		}
		return nil, fmt.Errorf("%w; creation outcome is unconfirmed; "+
			"inspect parent metadata before retrying; reserved output: %s",
			identitySecretAPIError("create API key", err), outputPath)
	}
	if issued == nil || status != http.StatusCreated {
		if output == nil {
			return nil, fmt.Errorf("missing API-key creation response; inspect parent metadata before retrying")
		}
		return nil, fmt.Errorf("missing API-key creation response; "+
			"inspect parent metadata before retrying; reserved output: %s", outputPath)
	}
	if util.IsValidUUID(issued.ID) {
		metadata.APIKeyID = issued.ID
	}
	if output != nil {
		if err := output.Capture(metadata, issued.Secret); err != nil {
			return nil, fmt.Errorf("API-key capture is unconfirmed: %w; inspect %s before retrying", err, outputPath)
		}
	}
	if options.Secret != nil && issued.Secret != *options.Secret {
		return nil, fmt.Errorf("API-key creation succeeded but differs from supplied material; inspect the issued record")
	}
	created, err := identityAPIKeyWriteObservation(raw, issued.ID)
	if err != nil {
		return nil, fmt.Errorf("API-key creation succeeded but public creation fields cannot be verified: %w", err)
	}
	expiration, supplied := options.ExpiresAt.Get()
	if !sameIdentityKeyName(created.Name, options.Name) || !maps.Equal(created.ManagedBy, options.ManagedBy) ||
		supplied && !sameIdentityCredentialExpiration(created.ExpiresAt, expiration) {
		return nil, fmt.Errorf("API-key creation succeeded but requested public fields were not retained")
	}
	observed, err := c.Get(ctx, created.ID)
	if err != nil {
		return nil, fmt.Errorf(
			"API-key creation succeeded but read-back failed; inspect metadata before retrying: %w", err,
		)
	}
	if !sameIdentityAPIKey(created, observed) {
		return nil, fmt.Errorf("API-key creation succeeded but public read-back differs; inspect the issued record")
	}
	return observed, nil
}

// Update explicitly preserves expiration for compatibility with older deployments.
// Management patches merge keys; explicit null values remove them.
func (c *IdentityAPIKeyClient) Update(ctx context.Context, id string, options IdentityAPIKeyUpdate) (
	*IdentityAPIKey, error,
) {
	expiration, supplied := options.ExpiresAt.Get()
	nameSupplied := options.NameSet || options.Name != nil
	if !nameSupplied && !supplied && options.ManagedBy == nil {
		return nil, fmt.Errorf("provide a name, expiration or management metadata change")
	}
	current, err := c.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !supplied {
		expiration = current.ExpiresAt
	}
	body := kkComps.UpdatePrincipalKeyBody{ExpiresAt: optionalnullable.From(expiration), ManagedBy: options.ManagedBy}
	if nameSupplied {
		body.Name = optionalnullable.From(options.Name)
	}
	var raw *http.Response
	status := 0
	ctx = httpclient.WithoutRetries(ctx)
	if c.group != nil {
		response, requestErr := c.group.UpdateGroupKey(ctx, kkOps.UpdateGroupKeyRequest{
			DirectoryID: c.directoryID, GroupID: c.parentID, APIKeyID: id, UpdatePrincipalKeyBody: body,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
		err = requestErr
		if response != nil {
			raw, status = response.RawResponse, response.StatusCode
		}
	} else {
		response, requestErr := c.principal.UpdatePrincipalKey(ctx, kkOps.UpdatePrincipalKeyRequest{
			DirectoryID: c.directoryID, PrincipalID: c.parentID, APIKeyID: id, UpdatePrincipalKeyBody: body,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
		err = requestErr
		if response != nil {
			raw, status = response.RawResponse, response.StatusCode
		}
	}
	if err != nil {
		return nil, identitySecretAPIError("update API key; inspect metadata before retrying", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("missing API-key update response; inspect metadata before retrying")
	}
	updated, err := identityAPIKeyWriteObservation(raw, id)
	if err != nil {
		return nil, err
	}
	expected := *current
	expected.ExpiresAt = expiration
	if nameSupplied {
		expected.Name = options.Name
	}
	expected.ManagedBy = maps.Clone(current.ManagedBy)
	for key, value := range options.ManagedBy {
		if value == nil {
			delete(expected.ManagedBy, key)
		} else {
			if expected.ManagedBy == nil {
				expected.ManagedBy = make(map[string]string)
			}
			expected.ManagedBy[key] = *value
		}
	}
	if !sameIdentityAPIKey(&expected, updated) {
		return nil, fmt.Errorf("API-key update did not retain requested fields and existing metadata")
	}
	observed, err := c.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sameIdentityAPIKey(&expected, observed) {
		return nil, fmt.Errorf("API-key update read-back differs from requested fields or existing metadata")
	}
	return observed, nil
}

func (c *IdentityAPIKeyClient) Delete(ctx context.Context, id string) error {
	if _, err := c.Get(ctx, id); err != nil {
		return err
	}
	values, err := c.List(ctx, 100)
	if err != nil {
		return err
	}
	found := false
	for _, value := range values {
		if value.ID == id {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("API-key ID is not a member of the selected parent collection")
	}
	ctx = httpclient.WithoutRetries(ctx)
	status := 0
	if c.group != nil {
		response, requestErr := c.group.DeleteGroupKey(ctx, kkOps.DeleteGroupKeyRequest{
			DirectoryID: c.directoryID, GroupID: c.parentID, APIKeyID: id,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
		err = requestErr
		if response != nil {
			status = response.StatusCode
		}
	} else {
		response, requestErr := c.principal.DeletePrincipalKey(ctx, kkOps.DeletePrincipalKeyRequest{
			DirectoryID: c.directoryID, PrincipalID: c.parentID, APIKeyID: id,
		}, kkOps.WithRetries(retry.Config{Strategy: httpclient.RetryStrategyNone}))
		err = requestErr
		if response != nil {
			status = response.StatusCode
		}
	}
	if err != nil {
		return identitySecretAPIError("delete API key; inspect metadata before retrying", err)
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("missing API-key deletion response; inspect metadata before retrying")
	}
	values, err = c.List(ctx, 100)
	if err != nil {
		return err
	}
	for _, value := range values {
		if value.ID == id {
			return fmt.Errorf("API key is still present after deletion")
		}
	}
	return nil
}
