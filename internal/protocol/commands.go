// Package protocol implements RTMP protocol command handling.
// This package provides command message parsing and generation
// for RTMP protocol operations like "connect", "publish", "play", etc.
package protocol

import (
	"bytes"
	"fmt"

	"github.com/DMA-Software/dma-gortmp/internal/amf"
)

// CommandName represents the name of an RTMP command.
type CommandName string

// Common RTMP command names
const (
	CommandConnect       CommandName = "connect"
	CommandCall          CommandName = "call"
	CommandClose         CommandName = "close"
	CommandCreateStream  CommandName = "createStream"
	CommandPlay          CommandName = "play"
	CommandPlay2         CommandName = "play2"
	CommandDeleteStream  CommandName = "deleteStream"
	CommandCloseStream   CommandName = "closeStream"
	CommandReceiveAudio  CommandName = "receiveAudio"
	CommandReceiveVideo  CommandName = "receiveVideo"
	CommandPublish       CommandName = "publish"
	CommandSeek          CommandName = "seek"
	CommandPause         CommandName = "pause"
	CommandResult        CommandName = "_result"
	CommandError         CommandName = "_error"
	CommandOnStatus      CommandName = "onStatus"
	CommandReleaseStream CommandName = "releaseStream"
	CommandFCPublish     CommandName = "FCPublish"
	CommandFCUnpublish   CommandName = "FCUnpublish"
	CommandOnBWDone      CommandName = "onBWDone"
)

// Command represents a parsed RTMP command message.
type Command struct {
	Name           CommandName
	TransactionID  float64
	CommandObject  interface{}
	AdditionalArgs []interface{}
}

// ConnectCommand represents the connect command parameters.
type ConnectCommand struct {
	App            string                 `amf:"app"`
	Type           string                 `amf:"type"`
	FlashVer       string                 `amf:"flashVer"`
	SwfUrl         string                 `amf:"swfUrl"`
	TcUrl          string                 `amf:"tcUrl"`
	Fpad           bool                   `amf:"fpad"`
	AudioCodecs    float64                `amf:"audioCodecs"`
	VideoCodecs    float64                `amf:"videoCodecs"`
	VideoFunction  float64                `amf:"videoFunction"`
	PageUrl        string                 `amf:"pageUrl"`
	ObjectEncoding float64                `amf:"objectEncoding"`
	Capabilities   float64                `amf:"capabilities"`
	Additional     map[string]interface{} `amf:",inline"`
}

// CreateStreamCommand represents the createStream command.
type CreateStreamCommand struct {
	// createStream has no additional parameters beyond transaction ID
}

// PublishCommand represents the publish command parameters.
type PublishCommand struct {
	StreamName string `json:"streamName"`
	Type       string `json:"type"` // "live", "record", "append"
}

// PlayCommand represents the play command parameters.
type PlayCommand struct {
	StreamName string  `json:"streamName"`
	Start      float64 `json:"start"`    // Start time in seconds, -2 for live
	Duration   float64 `json:"duration"` // Duration in seconds, -1 for unlimited
	Reset      bool    `json:"reset"`    // Whether to reset the playlist
}

// StatusCode represents RTMP status codes.
type StatusCode string

// Common RTMP status codes
const (
	StatusNetConnectionConnectSuccess  StatusCode = "NetConnection.Connect.Success"
	StatusNetConnectionConnectFailed   StatusCode = "NetConnection.Connect.Failed"
	StatusNetConnectionConnectClosed   StatusCode = "NetConnection.Connect.Closed"
	StatusNetConnectionConnectRejected StatusCode = "NetConnection.Connect.Rejected"
	StatusNetStreamPublishStart        StatusCode = "NetStream.Publish.Start"
	StatusNetStreamPublishFailed       StatusCode = "NetStream.Publish.Failed"
	StatusNetStreamPublishDenied       StatusCode = "NetStream.Publish.Denied"
	StatusNetStreamUnpublishSuccess    StatusCode = "NetStream.Unpublish.Success"
	StatusNetStreamPlayStart           StatusCode = "NetStream.Play.Start"
	StatusNetStreamPlayStop            StatusCode = "NetStream.Play.Stop"
	StatusNetStreamPlayFailed          StatusCode = "NetStream.Play.Failed"
	StatusNetStreamPlayStreamNotFound  StatusCode = "NetStream.Play.StreamNotFound"
	StatusNetStreamPlayReset           StatusCode = "NetStream.Play.Reset"
	StatusNetStreamSeekNotify          StatusCode = "NetStream.Seek.Notify"
	StatusNetStreamSeekFailed          StatusCode = "NetStream.Seek.Failed"
	StatusNetStreamPauseNotify         StatusCode = "NetStream.Pause.Notify"
	StatusNetStreamUnpauseNotify       StatusCode = "NetStream.Unpause.Notify"
	StatusNetStreamRecordStart         StatusCode = "NetStream.Record.Start"
	StatusNetStreamRecordStop          StatusCode = "NetStream.Record.Stop"
	StatusNetStreamRecordFailed        StatusCode = "NetStream.Record.Failed"
	StatusNetStreamRecordAlreadyExists StatusCode = "NetStream.Record.AlreadyExists"
)

// StatusLevel represents the level of a status message.
type StatusLevel string

// Status levels
const (
	StatusLevelStatus  StatusLevel = "status"
	StatusLevelError   StatusLevel = "error"
	StatusLevelWarning StatusLevel = "warning"
)

// StatusObject represents a status object in RTMP responses.
type StatusObject struct {
	Level       StatusLevel `json:"level"`
	Code        StatusCode  `json:"code"`
	Description string      `json:"description"`
	Details     string      `json:"details,omitempty"`
}

// CommandParser parses RTMP command messages from AMF data.
type CommandParser struct{}

// NewCommandParser creates a new command parser.
func NewCommandParser() *CommandParser {
	return &CommandParser{}
}

// ParseCommand parses a command from AMF0 encoded data.
func (p *CommandParser) ParseCommand(data []byte) (*Command, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty command data")
	}

	decoder := amf.NewAMF0Decoder(bytes.NewReader(data))

	// First element: command name (string)
	nameValue, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode command name: %w", err)
	}

	name, ok := nameValue.(string)
	if !ok {
		return nil, fmt.Errorf("command name is not a string: %T", nameValue)
	}

	// Second element: transaction ID (number)
	transactionValue, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode transaction ID: %w", err)
	}

	transactionID, ok := transactionValue.(float64)
	if !ok {
		return nil, fmt.Errorf("transaction ID is not a number: %T", transactionValue)
	}

	command := &Command{
		Name:          CommandName(name),
		TransactionID: transactionID,
	}

	// Third element: command object (maybe null)
	commandObjValue, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode command object: %w", err)
	}
	command.CommandObject = commandObjValue

	// Additional arguments
	var additionalArgs []interface{}
	for {
		arg, err := decoder.Decode()
		if err != nil {
			break // End of data
		}
		additionalArgs = append(additionalArgs, arg)
	}
	command.AdditionalArgs = additionalArgs

	return command, nil
}

// ParseConnectCommand parses a connect command from the command object.
func (p *CommandParser) ParseConnectCommand(commandObj interface{}) (*ConnectCommand, error) {
	obj, ok := commandObj.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("connect command object is not a map")
	}

	connect := &ConnectCommand{
		Additional: make(map[string]interface{}),
	}

	// Extract known fields
	if app, exists := obj["app"]; exists {
		if appStr, ok := app.(string); ok {
			connect.App = appStr
		}
	}

	if connType, exists := obj["type"]; exists {
		if typeStr, ok := connType.(string); ok {
			connect.Type = typeStr
		}
	}

	if flashVer, exists := obj["flashVer"]; exists {
		if flashStr, ok := flashVer.(string); ok {
			connect.FlashVer = flashStr
		}
	}

	if swfUrl, exists := obj["swfUrl"]; exists {
		if swfStr, ok := swfUrl.(string); ok {
			connect.SwfUrl = swfStr
		}
	}

	if tcUrl, exists := obj["tcUrl"]; exists {
		if tcStr, ok := tcUrl.(string); ok {
			connect.TcUrl = tcStr
		}
	}

	if fpad, exists := obj["fpad"]; exists {
		if fpadBool, ok := fpad.(bool); ok {
			connect.Fpad = fpadBool
		}
	}

	if audioCodecs, exists := obj["audioCodecs"]; exists {
		if audioNum, ok := audioCodecs.(float64); ok {
			connect.AudioCodecs = audioNum
		}
	}

	if videoCodecs, exists := obj["videoCodecs"]; exists {
		if videoNum, ok := videoCodecs.(float64); ok {
			connect.VideoCodecs = videoNum
		}
	}

	if videoFunction, exists := obj["videoFunction"]; exists {
		if videoFuncNum, ok := videoFunction.(float64); ok {
			connect.VideoFunction = videoFuncNum
		}
	}

	if pageUrl, exists := obj["pageUrl"]; exists {
		if pageStr, ok := pageUrl.(string); ok {
			connect.PageUrl = pageStr
		}
	}

	if objectEncoding, exists := obj["objectEncoding"]; exists {
		if objEncNum, ok := objectEncoding.(float64); ok {
			connect.ObjectEncoding = objEncNum
		}
	}

	if capabilities, exists := obj["capabilities"]; exists {
		if capNum, ok := capabilities.(float64); ok {
			connect.Capabilities = capNum
		}
	}

	// Store additional unknown fields
	knownFields := map[string]bool{
		"app": true, "type": true, "flashVer": true, "swfUrl": true,
		"tcUrl": true, "fpad": true, "audioCodecs": true, "videoCodecs": true,
		"videoFunction": true, "pageUrl": true, "objectEncoding": true,
		"capabilities": true,
	}

	for key, value := range obj {
		if !knownFields[key] {
			connect.Additional[key] = value
		}
	}

	return connect, nil
}

// ParsePublishCommand parses a publish command from arguments.
func (p *CommandParser) ParsePublishCommand(args []interface{}) (*PublishCommand, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("publish command requires at least 2 arguments")
	}

	streamName, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("publish stream name is not a string")
	}

	publishType, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("publish type is not a string")
	}

	return &PublishCommand{
		StreamName: streamName,
		Type:       publishType,
	}, nil
}

// ParsePlayCommand parses a play command from arguments.
func (p *CommandParser) ParsePlayCommand(args []interface{}) (*PlayCommand, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("play command requires at least 1 argument")
	}

	streamName, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("play stream name is not a string")
	}

	play := &PlayCommand{
		StreamName: streamName,
		Start:      -2, // Default to live
		Duration:   -1, // Default to unlimited
		Reset:      true,
	}

	// Optional start parameter
	if len(args) > 1 {
		if start, ok := args[1].(float64); ok {
			play.Start = start
		}
	}

	// Optional duration parameter
	if len(args) > 2 {
		if duration, ok := args[2].(float64); ok {
			play.Duration = duration
		}
	}

	// Optional reset parameter
	if len(args) > 3 {
		if reset, ok := args[3].(bool); ok {
			play.Reset = reset
		}
	}

	return play, nil
}

// CommandBuilder builds RTMP command messages.
type CommandBuilder struct{}

// NewCommandBuilder creates a new command builder.
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// BuildCommand builds a command message as AMF0 encoded data.
func (b *CommandBuilder) BuildCommand(name CommandName, transactionID float64, commandObj interface{}, args ...interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := amf.NewAMF0Encoder(&buf)

	// Encode command name
	if err := encoder.Encode(string(name)); err != nil {
		return nil, fmt.Errorf("failed to encode command name: %w", err)
	}

	// Encode transaction ID
	if err := encoder.Encode(transactionID); err != nil {
		return nil, fmt.Errorf("failed to encode transaction ID: %w", err)
	}

	// Encode command object
	if err := encoder.Encode(commandObj); err != nil {
		return nil, fmt.Errorf("failed to encode command object: %w", err)
	}

	// Encode additional arguments
	for i, arg := range args {
		if err := encoder.Encode(arg); err != nil {
			return nil, fmt.Errorf("failed to encode argument %d: %w", i, err)
		}
	}

	return buf.Bytes(), nil
}

// BuildConnectResult builds a connect result message.
func (b *CommandBuilder) BuildConnectResult(transactionID float64, properties map[string]interface{}, information map[string]interface{}) ([]byte, error) {
	return b.BuildCommand(CommandResult, transactionID, properties, information)
}

// BuildConnectError builds a connect error message.
func (b *CommandBuilder) BuildConnectError(transactionID float64, properties map[string]interface{}, information map[string]interface{}) ([]byte, error) {
	return b.BuildCommand(CommandError, transactionID, properties, information)
}

// BuildCreateStreamResult builds a createStream result message.
func (b *CommandBuilder) BuildCreateStreamResult(transactionID float64, streamID float64) ([]byte, error) {
	return b.BuildCommand(CommandResult, transactionID, nil, streamID)
}

// BuildOnStatus builds an onStatus message.
func (b *CommandBuilder) BuildOnStatus(transactionID float64, commandObj interface{}, status StatusObject) ([]byte, error) {
	statusMap := map[string]interface{}{
		"level":       string(status.Level),
		"code":        string(status.Code),
		"description": status.Description,
	}

	if status.Details != "" {
		statusMap["details"] = status.Details
	}

	return b.BuildCommand(CommandOnStatus, transactionID, commandObj, statusMap)
}

// BuildPublishResult builds a successful publish response.
func (b *CommandBuilder) BuildPublishResult(transactionID float64) ([]byte, error) {
	status := StatusObject{
		Level:       StatusLevelStatus,
		Code:        StatusNetStreamPublishStart,
		Description: "Stream is now published",
	}
	return b.BuildOnStatus(transactionID, nil, status)
}

// BuildPlayResult builds a successful play response.
func (b *CommandBuilder) BuildPlayResult(transactionID float64) ([]byte, error) {
	status := StatusObject{
		Level:       StatusLevelStatus,
		Code:        StatusNetStreamPlayStart,
		Description: "Stream is now playing",
	}
	return b.BuildOnStatus(transactionID, nil, status)
}

// BuildError builds a generic error response.
func (b *CommandBuilder) BuildError(transactionID float64, code StatusCode, description string) ([]byte, error) {
	status := StatusObject{
		Level:       StatusLevelError,
		Code:        code,
		Description: description,
	}
	return b.BuildOnStatus(transactionID, nil, status)
}
