package dd

import (
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
