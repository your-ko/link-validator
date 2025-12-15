package dd

import (
	"context"
	"net/http"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/stretchr/testify/mock"
)

// Ptr is a helper routine that allocates a new T value
// to store v and returns a pointer to it.
func Ptr[T any](v T) *T {
	return &v
}

func Test_handleMonitors(t *testing.T) {
	type args struct {
		resource ddResource
	}
	tests := []struct {
		name      string
		args      args
		setupMock func(*mockclient)
		wantErr   error
	}{
		{
			name: "Monitors list",
			args: args{resource: ddResource{Type: "monitors"}},
			setupMock: func(m *mockclient) {
				monitors := []datadogV1.Monitor{{Name: Ptr("test")}}
				resp := &http.Response{StatusCode: http.StatusOK}
				m.EXPECT().ListMonitors(mock.Anything).Return(monitors, resp, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleMonitors(context.Background(), mockClient, tt.args.resource)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			//if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
			//	t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			//}
		})
	}
}
