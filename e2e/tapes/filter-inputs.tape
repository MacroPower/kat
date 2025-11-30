Hide
  Source e2e/config.tape
  Output e2e/golden/filter-inputs.ascii

  Type "cat example/kustomize/resources.yaml | ./kat - --config=api/v1beta1/configs/config.yaml --trust" Enter
  Sleep 6s
Show

# List filter
Type "/"
Type "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
Left 26
Type ` !@#$%^&*()-_=+{}[]\|;':"<>?/~ `
Right 26
Escape

# Pager
Enter

# Pager search
Type "/"
Type "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
Left 26
Type ` !@#$%^&*()-_=+{}[]\|;':"<>?/~ `
Right 26
Escape

# Exit
Type "q"
