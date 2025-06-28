# VHS documentation
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>
#   Output <path>.mp4               Create an MP4 output at the given <path>
#   Output <path>.webm              Create a WebM output at the given <path>
#
# Require:
#   Require <string>                Ensure a program is on the $PATH to proceed
#
# Settings:
#   Set FontSize <number>           Set the font size of the terminal
#   Set FontFamily <string>         Set the font family of the terminal
#   Set Height <number>             Set the height of the terminal
#   Set Width <number>              Set the width of the terminal
#   Set LetterSpacing <float>       Set the font letter spacing (tracking)
#   Set LineHeight <float>          Set the font line height
#   Set LoopOffset <float>%         Set the starting frame offset for the GIF loop
#   Set Theme <json|string>         Set the theme of the terminal
#   Set Padding <number>            Set the padding of the terminal
#   Set Framerate <number>          Set the framerate of the recording
#   Set PlaybackSpeed <float>       Set the playback speed of the recording
#   Set MarginFill <file|#000000>   Set the file or color the margin will be filled with.
#   Set Margin <number>             Set the size of the margin. Has no effect if MarginFill isn't set.
#   Set BorderRadius <number>       Set terminal border radius, in pixels.
#   Set WindowBar <string>          Set window bar type. (one of: Rings, RingsRight, Colorful, ColorfulRight)
#   Set WindowBarSize <number>      Set window bar size, in pixels. Default is 40.
#   Set TypingSpeed <time>          Set the typing speed of the terminal. Default is 50ms.
#
# Sleep:
#   Sleep <time>                    Sleep for a set amount of <time> in seconds
#
# Type:
#   Type[@<time>] "<characters>"    Type <characters> into the terminal with a
#                                   <time> delay between each character
#
# Keys:
#   Escape[@<time>] [number]        Press the Escape key
#   Backspace[@<time>] [number]     Press the Backspace key
#   Delete[@<time>] [number]        Press the Delete key
#   Insert[@<time>] [number]        Press the Insert key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   PageUp[@<time>] [number]        Press the Page Up key
#   PageDown[@<time>] [number]      Press the Page Down key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output

Output docs/assets/demo.gif

Set Shell "bash"
Set Padding 20
Set FontSize 22
Set Width 1680
Set Height 600

Hide

  Type `go build -o dist/kat -ldflags="-X github.com/macropower/kat/pkg/version.Version=0.x" cmd/kat/main.go`  Enter
  Type "alias kat=./dist/kat"  Enter
  Type "export KAT_UI_GLAMOUR_STYLE=dark"  Enter

  Sleep 500ms
  Type "clear"  Enter
  Sleep 500ms

  Type "tmux -f /dev/null -L test new-session -- bash" Sleep 500ms Enter
  Type "tmux split-window -d -h -- bash && \" Sleep 500ms Enter
  Type "tmux set status && \" Sleep 500ms Enter
  Type 'tmux setw pane-border-style "fg=0" && \' Sleep 500ms Enter
  Type 'tmux setw pane-active-border-style "fg=0"' Sleep 500ms Enter
  Sleep 500ms
  Ctrl+L

  Type "vim example/helm/values.yaml"  Enter
  Sleep 500ms
  Type ":set backupcopy=yes"  Enter
  Type ":set nowrap"  Enter

  Ctrl+B
  Type o
  Type "kat example/helm -w"  Enter
  Sleep 4s

Show

Sleep 500ms

Type "/"
Sleep 500ms
Type "deployment" Sleep 500ms Enter
Sleep 500ms
PageDown@100ms 1
Sleep 100ms
Down@50ms 5
Sleep 100ms

Ctrl+B
Type o

Type "/tag:" Sleep 500ms Enter
Type 'f"' Sleep 250ms
Type "a" Sleep 250ms
Type 'latest"' Sleep 500ms
Escape
Type ":w" Sleep 500ms Enter
Escape

Sleep 2s

Type "a" Sleep 500ms
Delete Sleep 250ms
Escape
Type ":w" Sleep 500ms Enter
Escape

Sleep 3s

Hide

Type ':%s/tag: "latest"/tag: ""/' Sleep 500ms Enter
Type ":wq!" Sleep 500ms Enter

Sleep 2s
