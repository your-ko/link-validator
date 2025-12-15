package dd

import (
	"context"
	"reflect"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

func TestLinkProcessor_ExtractLinks(t *testing.T) {
	type args struct {
		line string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "captures app datadog urls",
			args: args{line: `test
				https://app.datadoghq.com/metric/explorer?fromUser=false,
				https://app.datadoghq.com/monitors/manage,
				https://app.datadoghq.com/monitors/1234567890,
				https://app.datadoghq.com/on-call/teams,
				https://app.datadoghq.com/dashboard/somepath/somedashboard
				https://github.com/DataDog/datadog-api-client-go/,
				https://docs.datadoghq.com/,
				https://google.com,
				test`},
			want: []string{
				"https://app.datadoghq.com/metric/explorer?fromUser=false",
				"https://app.datadoghq.com/monitors/manage",
				"https://app.datadoghq.com/monitors/1234567890",
				"https://app.datadoghq.com/on-call/teams",
				"https://app.datadoghq.com/dashboard/somepath/somedashboard",
			},
		},
		{
			name: "ignores urls with special characters",
			args: args{line: `test
				https://app.datadoghq.com/monitors/[1234567890],
				https://app.datadoghq.com/[monitors],
				test`},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &LinkProcessor{}
			if got := proc.ExtractLinks(tt.args.line); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractLinks() = %v, want %v", got, tt.want)
			}
		})
	}
}

//func Test_parseUrl(t *testing.T) {
//	type args struct {
//		link string
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    interface{}
//		wantErr bool
//	}{
//		{
//			name:    "parses list monitors",
//			args:    args{link: "https://app.datadoghq.com/monitors"},
//			want:    nil,
//			wantErr: false,
//		},
//		{
//			name:    "parses list monitors with a query string",
//			args:    args{link: "https://app.datadoghq.com/monitors/manage?q=team%3A%28thebest&p=1"},
//			want:    nil,
//			wantErr: false,
//		},
//		{
//			name:    "parses particular monitor",
//			args:    args{link: "https://app.datadoghq.com/monitors/1234567890"},
//			want:    nil,
//			wantErr: false,
//		},
//		{
//			name:    "parses particular monitor edit",
//			args:    args{link: "https://app.datadoghq.com/monitors/1234567890/edit"},
//			want:    nil,
//			wantErr: false,
//		},
//		{
//			name:    "parses list dashboards",
//			args:    args{link: "https://app.datadoghq.com/dashboard/lists?p1"},
//			want:    nil,
//			wantErr: false,
//		},
//		{
//			name:    "parses particular dashboard",
//			args:    args{link: "https://app.datadoghq.com/dashboard/12345/somedashboard?fromUser=false"},
//			want:    nil,
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := parseUrl(tt.args.link)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("parseUrl() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("parseUrl() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

func TestLinkProcessor_Process(t *testing.T) {
	type fields struct {
		client *datadog.APIClient
	}
	type args struct {
		ctx  context.Context
		link string
		in2  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "parses particular monitor",
			args:    args{link: "https://app.datadoghq.com/monitors/1234567890"},
			wantErr: false,
		},
		{
			name:    "parses particular dashboard",
			args:    args{link: "https://app.datadoghq.com/dashboard/dashboardId/dashboardName"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &LinkProcessor{
				//client: tt.fields.client,
			}
			if err := proc.Process(tt.args.ctx, tt.args.link, tt.args.in2); (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
