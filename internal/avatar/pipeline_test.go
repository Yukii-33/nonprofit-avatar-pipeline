package avatar

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type recordingClient struct {
	calls      []string
	metadata   map[string]any
	resizeOpts ResizeOptions
	failAt     string
}

func (c *recordingClient) Upload(_ context.Context, _ io.Reader, _ string, _ string) (string, error) {
	c.calls = append(c.calls, "upload")
	return "uploaded", nil
}

func (c *recordingClient) SmartCrop(_ context.Context, _ string, _ string, _ string) (string, error) {
	c.calls = append(c.calls, "crop")
	return "cropped", nil
}

func (c *recordingClient) Resize(_ context.Context, _ string, opts ResizeOptions, _ string) (string, error) {
	c.calls = append(c.calls, "resize")
	c.resizeOpts = opts
	return "final-avatar", nil
}

func (c *recordingClient) UpdateUser(_ context.Context, _ string, metadata map[string]any, _ string) error {
	c.calls = append(c.calls, "update-user")
	c.metadata = metadata
	if c.failAt == "update-user" {
		return errors.New("rejected")
	}
	return nil
}

func TestPipelineAssignsAvatarUseByNonprofitProfile(t *testing.T) {
	tests := []struct {
		name string
		kind ProfileKind
		use  string
	}{
		{name: "donor receipts", kind: DonorProfile, use: "donor_receipts"},
		{name: "volunteer reminders", kind: VolunteerProfile, use: "volunteer_reminders"},
		{name: "campaign reporting", kind: CampaignProfile, use: "campaign_reporting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingClient{}
			result, err := NewPipeline(client).Process(context.Background(), UploadInput{
				UserID: "user-42", Filename: "avatar.png", Content: strings.NewReader("image"),
				ProfileKind: tt.kind, RequestID: "request-42",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.ProfileUse != tt.use || client.metadata["avatar_profile_use"] != tt.use {
				t.Fatalf("profile use = %q, metadata = %q", result.ProfileUse, client.metadata["avatar_profile_use"])
			}
			if got := strings.Join(client.calls, ","); got != "upload,crop,resize,update-user" {
				t.Fatalf("call order = %s", got)
			}
			if !client.resizeOpts.Store || client.resizeOpts.Format != "webp" || client.resizeOpts.Width != 512 {
				t.Fatalf("resize options = %+v", client.resizeOpts)
			}
		})
	}
}

func TestPipelineRejectsUnknownProfileBeforeUpload(t *testing.T) {
	client := &recordingClient{}
	_, err := NewPipeline(client).Process(context.Background(), UploadInput{ProfileKind: "board"})
	if err == nil || len(client.calls) != 0 {
		t.Fatalf("error = %v, calls = %v", err, client.calls)
	}
}
