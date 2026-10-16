package avatar

import (
	"context"
	"fmt"
	"io"
)

type ProfileKind string

const (
	DonorProfile     ProfileKind = "donor"
	VolunteerProfile ProfileKind = "volunteer"
	CampaignProfile  ProfileKind = "campaign"
)

type UploadInput struct {
	UserID      string
	Filename    string
	Content     io.Reader
	ProfileKind ProfileKind
	RequestID   string
}

type UploadResult struct {
	ImageID    string `json:"image_id"`
	ProfileUse string `json:"profile_use"`
}

type InvalidInputError struct {
	Message string
}

func (e *InvalidInputError) Error() string { return e.Message }

type ImageClient interface {
	Upload(context.Context, io.Reader, string, string) (string, error)
	SmartCrop(context.Context, string, string, string) (string, error)
	Resize(context.Context, string, ResizeOptions, string) (string, error)
	UpdateUser(context.Context, string, map[string]any, string) error
}

type ResizeOptions struct {
	Width   int
	Height  int
	Fit     string
	Enlarge bool
	Format  string
	Store   bool
}

type Pipeline struct {
	client ImageClient
}

func NewPipeline(client ImageClient) *Pipeline {
	return &Pipeline{client: client}
}

func (p *Pipeline) Process(ctx context.Context, in UploadInput) (UploadResult, error) {
	profileUse, err := useForProfile(in.ProfileKind)
	if err != nil {
		return UploadResult{}, err
	}

	uploadedID, err := p.client.Upload(ctx, in.Content, in.Filename, in.RequestID+":upload")
	if err != nil {
		return UploadResult{}, fmt.Errorf("upload avatar: %w", err)
	}
	croppedID, err := p.client.SmartCrop(ctx, uploadedID, "1:1", in.RequestID+":crop")
	if err != nil {
		return UploadResult{}, fmt.Errorf("crop avatar: %w", err)
	}
	finalID, err := p.client.Resize(ctx, croppedID, ResizeOptions{
		Width: 512, Height: 512, Fit: "cover", Enlarge: false, Format: "webp", Store: true,
	}, in.RequestID+":optimize")
	if err != nil {
		return UploadResult{}, fmt.Errorf("optimize avatar: %w", err)
	}

	metadata := map[string]any{
		"avatar_image_id":    finalID,
		"avatar_profile_use": profileUse,
	}
	if err := p.client.UpdateUser(ctx, in.UserID, metadata, in.RequestID+":user"); err != nil {
		return UploadResult{}, fmt.Errorf("attach avatar to user: %w", err)
	}
	return UploadResult{ImageID: finalID, ProfileUse: profileUse}, nil
}

func useForProfile(kind ProfileKind) (string, error) {
	switch kind {
	case DonorProfile:
		return "donor_receipts", nil
	case VolunteerProfile:
		return "volunteer_reminders", nil
	case CampaignProfile:
		return "campaign_reporting", nil
	default:
		return "", &InvalidInputError{Message: "profile_kind must be donor, volunteer, or campaign"}
	}
}
