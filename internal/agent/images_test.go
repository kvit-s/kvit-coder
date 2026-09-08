package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// stubImageTool returns a fixed image attachment like ReadImage would.
type stubImageTool struct {
	part llm.ImagePart
}

type stubImageResult struct {
	Summary string          `json:"summary"`
	images  []llm.ImagePart `json:"-"`
}

func (r *stubImageResult) ToolImages() []llm.ImagePart { return r.images }

func (s *stubImageTool) Name() string        { return "ReadImage" }
func (s *stubImageTool) Description() string { return "test image tool" }
func (s *stubImageTool) JSONSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s *stubImageTool) Check(ctx context.Context, args json.RawMessage) error { return nil }
func (s *stubImageTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	return &stubImageResult{Summary: "a red square", images: []llm.ImagePart{s.part}}, nil
}
func (s *stubImageTool) PromptSection() string      { return "" }
func (s *stubImageTool) PromptCategory() string     { return "filesystem" }
func (s *stubImageTool) PromptOrder() int           { return 11 }
func (s *stubImageTool) PromptTemplateName() string { return "" }
func (s *stubImageTool) ParallelSafe() bool         { return true }

var _ tools.ImageCarrier = (*stubImageResult)(nil)

func TestReadImageAttachesFollowerMessage(t *testing.T) {
	part := llm.ImagePart{Path: "/tmp/shot.png", MediaType: "image/png", Width: 64, Height: 64, Data: []byte("pixels")}
	client := newFakeClient(
		calls(toolCall("call-1", "ReadImage", map[string]any{"path": "shot.png"})),
		answer("it is red"),
	)
	r, _ := newTestRunner(t, testConfig(), client, &stubImageTool{part: part})
	res, err := r.Run(context.Background(), RunConfig{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "what color?"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The second request carries the tool result plus the follower image.
	reqs := client.requests
	if len(reqs) < 2 {
		t.Fatalf("got %d requests, want at least 2", len(reqs))
	}
	msgs := reqs[1].Messages
	var toolMsg, followMsg *llm.Message
	for i := range msgs {
		if msgs[i].Role == llm.RoleTool {
			toolMsg = &msgs[i]
		}
	}
	for i := range msgs {
		if msgs[i].Role == llm.RoleUser && len(msgs[i].Images) > 0 {
			followMsg = &msgs[i]
		}
	}
	if toolMsg == nil {
		t.Fatal("no tool message in second request")
	}
	if len(toolMsg.Images) != 0 {
		t.Errorf("tool message carries %d images, want none (text only)", len(toolMsg.Images))
	}
	if !strings.Contains(toolMsg.Content, "red square") {
		t.Errorf("tool content = %q", toolMsg.Content)
	}
	if followMsg == nil {
		t.Fatal("no follower user message with images")
	}
	if len(followMsg.Images) != 1 || followMsg.Images[0].Path != "/tmp/shot.png" {
		t.Errorf("follower images = %+v", followMsg.Images)
	}
	if res.Stats == nil || len(res.FinalMessages) == 0 {
		t.Error("run produced no result")
	}
	// No pixels leak into the persisted text: the tool content is JSON.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(toolMsg.Content), &decoded); err != nil {
		t.Errorf("tool content is not JSON: %v", err)
	}
}

func TestImageContextGuardFailsEarly(t *testing.T) {
	cfg := testConfig()
	cfg.LLM.Context = 1000 // smaller than the image estimate alone
	part := llm.ImagePart{Path: "/tmp/big.png", MediaType: "image/png", Width: 2048, Height: 2048, Size: 1 << 20, Data: []byte("pixels")}
	client := newFakeClient(answer("unreachable"))
	r, out := newTestRunner(t, cfg, client, &stubImageTool{part: part})
	_, err := r.Run(context.Background(), RunConfig{
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: "look",
			Images:  []llm.ImagePart{part},
		}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if client.callCount() != 0 {
		t.Errorf("model was called %d times, want 0 (fail before the wire)", client.callCount())
	}
	if !strings.Contains(out.String(), "images need") {
		t.Errorf("no early-budget error in output:\n%s", out.String())
	}
}
