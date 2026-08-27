package main

import "testing"

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		mail    bool
		skip    bool
		wantErr bool
	}{
		{name: "site mode", mail: false, skip: false, wantErr: false},
		{name: "mail mode", mail: true, skip: false, wantErr: false},
		{name: "skip mail", mail: false, skip: true, wantErr: false},
		{name: "conflict", mail: true, skip: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOptions(tt.mail, tt.skip)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOptions(%v, %v) error = %v, wantErr %v", tt.mail, tt.skip, err, tt.wantErr)
			}
		})
	}
}
