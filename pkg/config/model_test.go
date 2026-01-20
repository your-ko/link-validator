package config

import "testing"

func TestDataDogConfig_validate(t *testing.T) {
	tests := []struct {
		name          string
		config        DataDogConfig
		wantErr       bool
		expectedError string
	}{
		{
			name: "datadog enabled without API keys should return error",
			config: DataDogConfig{
				Enabled: true,
				ApiKey:  "",
				AppKey:  "",
			},
			wantErr:       true,
			expectedError: "datadog validator is enabled but DD_API_KEY/DD_APP_KEY are not set",
		},
		{
			name: "datadog enabled with valid API keys should pass",
			config: DataDogConfig{
				Enabled: true,
				ApiKey:  "qwerty",
				AppKey:  "qwerty",
			},
			wantErr: false,
		},
		{
			name: "datadog disabled should pass validation regardless of keys",
			config: DataDogConfig{
				Enabled: false,
				ApiKey:  "",
				AppKey:  "",
			},
			wantErr: false,
		},
		{
			name: "datadog enabled with only API key should return error",
			config: DataDogConfig{
				Enabled: true,
				ApiKey:  "qwerty",
				AppKey:  "",
			},
			wantErr:       true,
			expectedError: "datadog validator is enabled but DD_API_KEY/DD_APP_KEY are not set",
		},
		{
			name: "datadog enabled with only App key should return error",
			config: DataDogConfig{
				Enabled: true,
				ApiKey:  "",
				AppKey:  "qwerty",
			},
			wantErr:       true,
			expectedError: "datadog validator is enabled but DD_API_KEY/DD_APP_KEY are not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("DataDogConfig.validate() expected error but got none")
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("DataDogConfig.validate() error = %v, expected %v", err.Error(), tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("DataDogConfig.validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestGitHubConfig_validate(t *testing.T) {
	tests := []struct {
		name          string
		config        GitHubConfig
		wantErr       bool
		expectedError string
	}{
		{
			name: "corporate GitHub URL without PAT should return error",
			config: GitHubConfig{
				Enabled:       true,
				PAT:           "pat",
				CorpGitHubUrl: "https://github.mycorp.com",
				CorpPAT:       "",
			},
			wantErr:       true,
			expectedError: "it seems you set CORP_URL but didn't provide CORP_PAT. Expect false negatives because the link-validator won't be able to fetch corl github without token",
		},
		{
			name: "valid corporate GitHub configuration should pass",
			config: GitHubConfig{
				Enabled:       true,
				PAT:           "pat",
				CorpGitHubUrl: "https://github.mycorp.com",
				CorpPAT:       "corp-pat",
			},
			wantErr: false,
		},
		{
			name: "no corporate GitHub configuration should pass",
			config: GitHubConfig{
				Enabled:       true,
				PAT:           "pat",
				CorpGitHubUrl: "",
				CorpPAT:       "",
			},
			wantErr: false,
		},
		{
			name: "corporate PAT without URL should pass validation",
			config: GitHubConfig{
				Enabled:       false,
				PAT:           "",
				CorpGitHubUrl: "",
				CorpPAT:       "corp-pat",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("GitHubConfig.validate() expected error but got none")
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("GitHubConfig.validate() error = %v, expected %v", err.Error(), tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("GitHubConfig.validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestHttpConfig_validate(t *testing.T) {
	tests := []struct {
		name          string
		config        HttpConfig
		wantErr       bool
		expectedError string
	}{
		{
			name: "Redirects is positive. Passing",
			config: HttpConfig{
				Enabled:   true,
				Redirects: 3,
				Ignore:    []string{},
			},
			wantErr: false,
		},
		{
			name: "Redirects is negative. Config is disabled. Passing",
			config: HttpConfig{
				Enabled:   false,
				Redirects: -3,
				Ignore:    []string{},
			},
			wantErr: false,
		},
		{
			name: "Redirects is negative. Failing",
			config: HttpConfig{
				Enabled:   true,
				Redirects: -3,
				Ignore:    []string{},
			},
			wantErr:       true,
			expectedError: "redirects should be a positive integer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("HttpConfig.validate() expected error but got none")
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("HttpConfig.validate() error = %v, expected %v", err.Error(), tt.expectedError)
				}
			} else {
				if err != nil {
					t.Errorf("HttpConfig.validate() unexpected error = %v", err)
				}
			}
		})
	}
}
