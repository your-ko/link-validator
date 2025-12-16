package dd

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
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
			args: args{resource: ddResource{typ: "monitors"}},
			setupMock: func(m *mockclient) {
				monitors := []datadogV1.Monitor{{Name: Ptr("test")}}
				resp := &http.Response{StatusCode: http.StatusOK}
				m.EXPECT().ListMonitors(mock.Anything).Return(monitors, resp, nil)
			},
		},
		{
			name: "Particular monitor",
			args: args{resource: ddResource{
				typ: "monitors",
				id:  "1234567890",
			}},
			setupMock: func(m *mockclient) {
				monitor := datadogV1.Monitor{Name: Ptr("test"), Id: Ptr(int64(1234567890))}
				resp := &http.Response{StatusCode: http.StatusOK}
				m.EXPECT().GetMonitor(mock.Anything, int64(1234567890)).Return(monitor, resp, nil)
			},
		},
		{
			name: "Particular monitor incorrect id",
			args: args{resource: ddResource{
				typ: "monitors",
				id:  "qwerty",
			}},
			setupMock: func(m *mockclient) {},
			wantErr:   errors.New("invalid monitor id: 'qwerty'"),
		},
		{
			name: "monitor not found",
			args: args{resource: ddResource{
				typ: "monitors",
				id:  "1234567890",
			}},
			setupMock: func(m *mockclient) {
				monitor := datadogV1.Monitor{}
				err := datadog.GenericOpenAPIError{
					ErrorMessage: "404 Not Found",
					ErrorModel:   []string{"Monitor not found"},
				}
				resp := &http.Response{StatusCode: http.StatusNotFound}
				m.EXPECT().GetMonitor(mock.Anything, int64(1234567890)).Return(monitor, resp, err)
			},
			wantErr: datadog.GenericOpenAPIError{
				ErrorMessage: "404 Not Found",
				ErrorModel:   []string{"Monitor not found"},
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

func Test_handleDashboards(t *testing.T) {
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
			name: "Particular dashboard",
			args: args{resource: ddResource{typ: "dashboard", id: "1234567890"}},
			setupMock: func(m *mockclient) {
				dashboard := datadogV1.Dashboard{Title: "title"}
				resp := &http.Response{StatusCode: http.StatusOK}
				m.EXPECT().GetDashboard(mock.Anything, "1234567890").Return(dashboard, resp, nil)
			},
		},
		{
			name: "dashboard not found",
			args: args{resource: ddResource{typ: "dashboard", id: "1234567890"}},
			setupMock: func(m *mockclient) {
				dashboard := datadogV1.Dashboard{}
				err := datadog.GenericOpenAPIError{
					ErrorMessage: "404 Not Found",
					ErrorModel:   []string{"Not found"},
				}
				resp := &http.Response{StatusCode: http.StatusNotFound}
				m.EXPECT().GetDashboard(mock.Anything, "1234567890").Return(dashboard, resp, err)
			},
			wantErr: datadog.GenericOpenAPIError{
				ErrorMessage: "404 Not Found",
				ErrorModel:   []string{"Not found"},
			},
		},
		{
			name: "Manual preset list",
			args: args{resource: ddResource{typ: "dashboard", id: "1234567890", subType: "lists/manual"}},
			setupMock: func(m *mockclient) {
				dashboards := datadogV1.DashboardList{
					Name: "manual list",
					Type: Ptr("manual_dashboard_list"),
				}
				resp := &http.Response{StatusCode: http.StatusOK}
				m.EXPECT().GetDashboardList(mock.Anything, int64(1234567890)).Return(dashboards, resp, nil)
			},
		},
		{
			name: "Manual preset list not found",
			args: args{resource: ddResource{typ: "dashboard", id: "1234567890", subType: "lists/manual"}},
			setupMock: func(m *mockclient) {
				dashboards := datadogV1.DashboardList{}
				err := datadog.GenericOpenAPIError{
					ErrorMessage: "404 Not Found",
					ErrorModel:   []string{"Manual Dashboard List with id 1234567890 not found"},
				}

				resp := &http.Response{StatusCode: http.StatusNotFound}
				m.EXPECT().GetDashboardList(mock.Anything, int64(1234567890)).Return(dashboards, resp, err)
			},
			wantErr: datadog.GenericOpenAPIError{
				ErrorMessage: "404 Not Found",
				ErrorModel:   []string{"Manual Dashboard List with id 1234567890 not found"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleDashboards(context.Background(), mockClient, tt.args.resource)

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
