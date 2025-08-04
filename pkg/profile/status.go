package profile

import (
	"context"
	"errors"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type (
	// RenderStage represents the different stages of rendering.
	// It is used to track the progress of rendering operations.
	RenderStage int

	// RenderResult represents the result of a rendering operation.
	RenderResult string
)

const (
	// StageNone indicates no rendering is active.
	StageNone RenderStage = iota
	// StageInit is the initial stage of rendering. Not currently implemented.
	StageInit
	// StagePreRender is the stage before the main rendering command.
	StagePreRender
	// StageRender is the main rendering stage where the command is executed.
	StageRender
	// StagePostRender is the stage after the main rendering command.
	StagePostRender

	// ResultOK indicates the rendering was successful.
	ResultOK RenderResult = "OK"
	// ResultError indicates there was an error during rendering.
	ResultError RenderResult = "ERROR"
	// ResultCancel indicates the rendering was canceled.
	ResultCancel RenderResult = "CANCEL"
	// ResultNone indicates no rendering result is available.
	ResultNone RenderResult = ""
)

type Status struct {
	renderResult RenderResult
	renderStage  RenderStage
	mu           sync.Mutex
}

func (s *Status) SetStage(stage RenderStage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.renderStage = stage
}

func (s *Status) SetResult(result RenderResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.renderStage = StageNone
	s.renderResult = result
}

func (s *Status) SetError(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.renderStage = StageNone

	if errors.Is(ctx.Err(), context.Canceled) {
		s.renderResult = ResultCancel
	} else {
		s.renderResult = ResultError
	}
}

// RenderMap returns a map of render values exposed to CEL.
func (s *Status) RenderMap() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]any{
		"stage":  int(s.renderStage),
		"result": string(s.renderResult),
	}
}

type renderLib struct{}

func RenderLib() cel.EnvOption {
	return cel.Lib(renderLib{})
}

func (renderLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("render", cel.MapType(cel.StringType, cel.DynType)),

		cel.Constant("render.STAGE_NONE", cel.IntType, types.Int(StageNone)),
		cel.Constant("render.STAGE_PRE_RENDER", cel.IntType, types.Int(StagePreRender)),
		cel.Constant("render.STAGE_RENDER", cel.IntType, types.Int(StageRender)),
		cel.Constant("render.STAGE_POST_RENDER", cel.IntType, types.Int(StagePostRender)),

		cel.Constant("render.RESULT_NONE", cel.StringType, types.String(string(ResultNone))),
		cel.Constant("render.RESULT_OK", cel.StringType, types.String(string(ResultOK))),
		cel.Constant("render.RESULT_CANCEL", cel.StringType, types.String(string(ResultCancel))),
		cel.Constant("render.RESULT_ERROR", cel.StringType, types.String(string(ResultError))),
	}
}

func (renderLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
