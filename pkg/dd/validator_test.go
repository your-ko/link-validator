package dd

import (
	"net/url"
	"reflect"
	"testing"
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

func Test_parseUrl(t *testing.T) {
	type args struct {
		link string
	}
	tests := []struct {
		name    string
		args    args
		want    *ddResource
		wantErr bool
	}{
		{
			name: "parses list monitors",
			args: args{link: "https://app.datadoghq.com/monitors"},
			want: &ddResource{
				typ:   "",
				query: url.Values{},
			},
		},
		{
			name: "parses list monitors with a query string",
			args: args{link: "https://app.datadoghq.com/monitors/manage?q=team%3A%28thebest&p=1"},
			want: &ddResource{
				typ:      "monitors",
				action:   "manage",
				query:    url.Values{"q": []string{"team:(thebest"}, "p": []string{"1"}},
				fragment: "",
			},
		},
		{
			name: "parses particular monitor",
			args: args{link: "https://app.datadoghq.com/monitors/1234567890"},
			want: &ddResource{
				typ:   "monitors",
				id:    "1234567890",
				query: url.Values{},
			},
		},
		{
			name: "parses particular monitor edit",
			args: args{link: "https://app.datadoghq.com/monitors/1234567890/edit"},
			want: &ddResource{
				typ:    "monitors",
				id:     "1234567890",
				action: "edit",
				query:  url.Values{},
			},
		},
		{
			name: "parses list dashboards",
			args: args{link: "https://app.datadoghq.com/dashboard/lists?p1"},
			want: &ddResource{
				typ:     "",
				subType: "",
				query:   url.Values{"p1": []string{""}},
			},
		},
		{
			name: "parses list dashboards",
			args: args{link: "https://app.datadoghq.com/dashboard/shared?p1"},
			want: &ddResource{
				typ:     "",
				subType: "",
				query:   url.Values{"p1": []string{""}},
			},
		},
		{
			name: "parses dashboard presets",
			args: args{link: "https://app.datadoghq.com/dashboard/lists/preset/5"},
			want: &ddResource{
				typ:     "",
				subType: "lists/preset",
				id:      "5",
				query:   url.Values{},
			},
		},
		{
			name: "parses dashboard manual presets",
			args: args{link: "https://app.datadoghq.com/dashboard/lists/manual/123456789"},
			want: &ddResource{
				typ:     "dashboard",
				subType: "lists/manual",
				id:      "123456789",
				query:   url.Values{},
			},
		},
		{
			name: "parses dashboard reports",
			args: args{link: "https://app.datadoghq.com/dashboard/reports"},
			want: &ddResource{
				typ:     "",
				subType: "",
				query:   url.Values{},
			},
		},
		{
			name: "parses particular dashboard",
			args: args{link: "https://app.datadoghq.com/dashboard/12345/somedashboard?fromUser=false"},
			want: &ddResource{
				typ:     "dashboard",
				id:      "12345",
				subType: "somedashboard",
				query:   url.Values{"fromUser": []string{"false"}},
			},
		},
		{
			name: "parses particular integration dashboard",
			args: args{link: "https://app.datadoghq.com/dash/integration/12345/tools-overview?fromUser=false"},
			want: &ddResource{
				typ:     "dash/integration",
				id:      "12345",
				subType: "",
				query:   url.Values{"fromUser": []string{"false"}},
			},
		},
		{
			name: "ddsql",
			args: args{link: "https://app.datadoghq.com/ddsql/editor"},
			want: &ddResource{
				typ:     "ddsql",
				subType: "editor",
				query:   url.Values{},
			},
		},
		{
			name: "notebook list",
			args: args{link: "https://app.datadoghq.com/notebook/list?tags=team"},
			want: &ddResource{
				typ:     "",
				subType: "list",
				query:   url.Values{"tags": []string{"team"}},
			},
		},
		{
			name: "notebook reports list",
			args: args{link: "https://app.datadoghq.com/notebook/reports"},
			want: &ddResource{
				typ:     "",
				subType: "reports",
				query:   url.Values{},
			},
		},
		{
			name: "notebook template gallery",
			args: args{link: "https://app.datadoghq.com/notebook/template-gallery"},
			want: &ddResource{
				typ:     "",
				subType: "template-gallery",
				query:   url.Values{},
			},
		},
		{
			name: "notebook particular template",
			args: args{link: "https://app.datadoghq.com/notebook/custom-template/12345/postmortem-template-ticket-number-incident-title"},
			want: &ddResource{
				typ:     "notebook",
				subType: "custom-template",
				id:      "12345",
				query:   url.Values{},
			},
		},
		{
			name: "particular notebook",
			args: args{link: "https://app.datadoghq.com/notebook/12345/postmortem"},
			want: &ddResource{
				typ:   "notebook",
				id:    "12345",
				query: url.Values{},
			},
		},
		{
			name: "list of sheets",
			args: args{link: "https://app.datadoghq.com/sheets"},
			want: &ddResource{
				typ:   "sheets",
				query: url.Values{},
			},
		},
		{
			name: "particular sheet",
			args: args{link: "https://app.datadoghq.com/sheets/qwe-rty-123"},
			want: &ddResource{
				typ:   "sheets",
				id:    "qwe-rty-123",
				query: url.Values{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDataDogURL(tt.args.link)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUrl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseUrl() \n"+
					" got: %+v\n"+
					"want: %+v", got, tt.want)
			}
		})
	}
}

//func TestLinkProcessor_Process(t *testing.T) {
//	type fields struct {
//		client *datadog.APIClient
//	}
//	type args struct {
//		ctx  context.Context
//		link string
//		in2  string
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		wantErr bool
//	}{
//		{
//			name:    "parses particular monitor",
//			args:    args{link: "https://app.datadoghq.com/monitors/1234567890"},
//			wantErr: false,
//		},
//		{
//			name:    "parses particular dashboard",
//			args:    args{link: "https://app.datadoghq.com/dashboard/dashboardId/dashboardName"},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			proc := &LinkProcessor{
//				//client: tt.fields.client,
//			}
//			if err := proc.Process(tt.args.ctx, tt.args.link, tt.args.in2); (err != nil) != tt.wantErr {
//				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}
