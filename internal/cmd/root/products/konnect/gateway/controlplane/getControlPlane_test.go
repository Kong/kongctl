package controlplane

// Things to test:
// - Does a command read it's configuration properly?
// - Does a command handle errors from APIs properly?
// - Does a command print the output in the expected format?
// - Does a command handle the input flags properly?
// - Does a command handle the input arguments properly?
// - Does a command handle configuration properly?
// - Does a command write output to the proper stream?
// - Does a command return the proper exit code?

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrs "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTextDisplayConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    kkComps.ControlPlane
		expected textDisplayRecord
	}{
		{
			name:  "empty",
			input: kkComps.ControlPlane{},
			expected: textDisplayRecord{
				ID:                   "n/a",
				Name:                 "n/a",
				Description:          "n/a",
				Labels:               "n/a",
				ControlPlaneEndpoint: "n/a",
				LocalCreatedTime:     time.Time{}.In(time.Local).Format("2006-01-02 15:04:05"),
				LocalUpdatedTime:     time.Time{}.In(time.Local).Format("2006-01-02 15:04:05"),
			},
		},
		{
			name: "simple",
			input: kkComps.ControlPlane{
				ID:          "foo",
				Name:        "bar",
				Description: kk.String("baz"),
				Config: kkComps.Config{
					ControlPlaneEndpoint: kk.String("qux"),
				},
				Labels: map[string]string{
					"qux":   "quux",
					"corge": "grault",
				},
				CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: textDisplayRecord{
				ID:                   "foo",
				Name:                 "bar",
				Description:          "baz",
				Labels:               "qux: quux, corge: grault",
				ControlPlaneEndpoint: "qux",
				LocalCreatedTime:     time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC).In(time.Local).Format("2006-01-02 15:04:05"),
				LocalUpdatedTime:     time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC).In(time.Local).Format("2006-01-02 15:04:05"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rv := controlPlaneToDisplayRecord(&test.input)
			if rv != test.expected {
				t.Errorf("expected %v, got %v", test.expected, rv)
			}
		})
	}
}

func TestRunGet(t *testing.T) {
	type cpInfo struct {
		ID string
	}

	type input struct {
		sdk    func() *helpers.MockControlPlaneAPI
		helper func() *cmd.MockHelper
	}

	tests := []struct {
		name        string
		cp          cpInfo
		inputs      func(cpInfo) input
		expectedErr bool
		assertions  func(*testing.T, cpInfo, *kkComps.ControlPlane)
	}{
		{
			name: "simple",
			cp: cpInfo{
				ID: "4d9b3f3e-7b1b-4b6b-8b1b-4b6b7b1b4b6b",
			},
			inputs: func(cp cpInfo) input {
				return input{
					sdk: func() *helpers.MockControlPlaneAPI {
						rv := helpers.NewMockControlPlaneAPI(t)
						rv.
							EXPECT().
							GetControlPlane(context.Background(), cp.ID).
							Return(
								&kkOps.GetControlPlaneResponse{
									ControlPlane: &kkComps.ControlPlane{
										ID: cp.ID,
									},
								},
								nil,
							)
						return rv
					},
					helper: func() *cmd.MockHelper {
						rv := cmd.NewMockHelper(t)
						rv.
							EXPECT().
							GetContext().
							Return(context.Background())
						return rv
					},
				}
			},
			expectedErr: false,
			assertions: func(t *testing.T, cp cpInfo, result *kkComps.ControlPlane) {
				assert.Equal(t, cp.ID, result.ID)
			},
		},
		{
			name: "error",
			cp: cpInfo{
				ID: "4d9b3f3e-7b1b-4b6b-8b1b-4b6b7b1b4b6b",
			},
			inputs: func(cp cpInfo) input {
				return input{
					sdk: func() *helpers.MockControlPlaneAPI {
						rv := helpers.NewMockControlPlaneAPI(t)
						rv.
							EXPECT().
							GetControlPlane(context.Background(), cp.ID).
							Return(
								nil,
								kkErrs.NewSDKError("unknown content-type received: foo", 400, "", nil),
							)
						return rv
					},
					helper: func() *cmd.MockHelper {
						rv := cmd.NewMockHelper(t)
						rv.
							EXPECT().
							GetCmd().
							Return(&cobra.Command{})
						rv.
							EXPECT().
							GetContext().
							Return(context.Background())
						return rv
					},
				}
			},
			expectedErr: true,
			assertions: func(t *testing.T, _ cpInfo, result *kkComps.ControlPlane) {
				assert.Nil(t, result)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs := test.inputs(test.cp)
			sdk := inputs.sdk()
			helper := inputs.helper()
			result, err := runGet(test.cp.ID, sdk, helper)
			t.Cleanup(func() {
				assert.True(t, sdk.AssertExpectations(t))
			})

			test.assertions(t, test.cp, result)
			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TODO: Figure out how to make this all __far__ easier to write for future tests
func TestGetControlPlaneCmd(t *testing.T) {
	tests := []struct {
		Name        string
		Cmd         *cobra.Command
		Setup       func() (context.Context, *helpers.MockControlPlaneAPI)
		Args        []string
		expectedErr bool
		assertions  func(*testing.T, context.Context)
	}{
		{
			Name: "get-by-id",
			Cmd: func() *cobra.Command {
				baseCmd, _ := NewControlPlaneCmd(verbs.Get)
				newGetControlPlaneCmd(baseCmd)
				return baseCmd
			}(),
			Setup: func() (context.Context, *helpers.MockControlPlaneAPI) {
				ctx := context.Background()
				mockCPAPI := helpers.NewMockControlPlaneAPI(t)
				mockCPAPI.
					EXPECT().
					ListControlPlanes(mock.Anything, mock.Anything).
					Return(&kkOps.ListControlPlanesResponse{
						ListControlPlanesResponse: &kkComps.ListControlPlanesResponse{
							Data: []kkComps.ControlPlane{
								{
									ID:          "4d9b3f3e-7b1b-4b6b-8b1b-4b6b7b1b4b6b",
									Name:        "foo",
									Description: kk.String("blah"),
									Config: kkComps.Config{
										ControlPlaneEndpoint: kk.String("https://foo.bar"),
									},
								},
							},
						},
					}, nil)

				token := "super-duper-secret" // #nosec G101

				cfg := config.BuildProfiledConfig("default", "", viper.New())
				cfg.Set(kkCommon.RequestPageSizeConfigPath, 10)
				cfg.Set(common.OutputConfigPath, "text")
				cfg.Set(kkCommon.PATConfigPath, token)
				ctx = context.WithValue(ctx, config.ConfigKey, cfg)

				logger := slog.Default()
				ctx = context.WithValue(ctx, log.LoggerKey, logger)

				ctx = context.WithValue(ctx, iostreams.StreamsKey, iostreams.NewTestIOStreamsOnly())

				mockSDK := helpers.MockKonnectSDK{
					T:     t,
					Token: token,
					CPAPIFactory: func() helpers.ControlPlaneAPI {
						return mockCPAPI
					},
				}

				ctx = context.WithValue(ctx, helpers.SDKFactoryKey, helpers.SDKFactory(func(string) (helpers.SDKAPI, error) {
					return &mockSDK, nil
				}))

				return ctx, mockCPAPI
			},
			Args:        []string{"4d9b3f3e-7b1b-4b6b-8b1b-4b6b7b1b4b6b"},
			expectedErr: false,
			assertions: func(_ *testing.T, ctx context.Context) {
				out := ctx.Value(iostreams.StreamsKey).(*iostreams.IOStreams).Out
				result := out.(*bytes.Buffer).String()
				assert.True(t, strings.Contains(result, "4d9b3f3e-7b1b-4b6b-8b1b-4b6b7b1b4b6b"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(_ *testing.T) {
			ctx, sdk := test.Setup()

			err := test.Cmd.ExecuteContext(ctx)

			t.Cleanup(func() {
				assert.True(t, sdk.AssertExpectations(t))
			})

			test.assertions(t, ctx)

			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
