package main

import (
	"fmt"
	"io"
)

const bashCompletion = `_gochecks() {
  local cur=${COMP_WORDS[COMP_CWORD]} prev=${COMP_WORDS[COMP_CWORD-1]}
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=($(compgen -W "init run doctor version completion help" -- "$cur"))
    return
  fi
  case ${COMP_WORDS[1]} in
    run)
      if [ "$prev" = "--format" ] || [ "$prev" = "-format" ]; then
        COMPREPLY=($(compgen -W "text json" -- "$cur"))
      else
        COMPREPLY=($(compgen -W "--all --no-tui --format --jobs --help" -- "$cur"))
      fi ;;
    completion) COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "$cur")) ;;
    help) COMPREPLY=($(compgen -W "init run doctor version completion" -- "$cur")) ;;
  esac
}
complete -F _gochecks gochecks
`

const zshCompletion = `#compdef gochecks
_gochecks() {
  local -a cmds
  cmds=(
    'init:Write default configs and install the git pre-commit hook'
    'run:Run Checks on staged packages'
    'doctor:Report which wrapped tools are installed'
    'version:Print the version'
    'completion:Print a shell completion script'
    'help:Show help for a command'
  )
  if (( CURRENT == 2 )); then
    _describe 'command' cmds
    return
  fi
  case $words[2] in
    run) _arguments \
      '--all[analyse the whole module and include tests]' \
      '--no-tui[plain output even on a terminal]' \
      '--format[output format]:format:(text json)' \
      '--jobs[max Checks running at once]:jobs:' ;;
    completion) _values 'shell' bash zsh fish powershell ;;
    help) _values 'command' init run doctor version completion ;;
  esac
}
compdef _gochecks gochecks
`

const fishCompletion = `complete -c gochecks -f
complete -c gochecks -n __fish_use_subcommand -a init -d 'Write default configs and install the git pre-commit hook'
complete -c gochecks -n __fish_use_subcommand -a run -d 'Run Checks on staged packages'
complete -c gochecks -n __fish_use_subcommand -a doctor -d 'Report which wrapped tools are installed'
complete -c gochecks -n __fish_use_subcommand -a version -d 'Print the version'
complete -c gochecks -n __fish_use_subcommand -a completion -d 'Print a shell completion script'
complete -c gochecks -n __fish_use_subcommand -a help -d 'Show help for a command'
complete -c gochecks -n '__fish_seen_subcommand_from run' -l all -d 'Analyse the whole module and include tests'
complete -c gochecks -n '__fish_seen_subcommand_from run' -l no-tui -d 'Plain output even on a terminal'
complete -c gochecks -n '__fish_seen_subcommand_from run' -l format -x -a 'text json' -d 'Output format'
complete -c gochecks -n '__fish_seen_subcommand_from run' -l jobs -x -d 'Max Checks running at once'
complete -c gochecks -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell'
complete -c gochecks -n '__fish_seen_subcommand_from help' -a 'init run doctor version completion'
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName gochecks -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $words = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
  $candidates = if ($words.Count -le 1 -or ($words.Count -eq 2 -and $wordToComplete)) {
    'init', 'run', 'doctor', 'version', 'completion', 'help'
  } elseif ($words[1] -eq 'run') {
    '--all', '--no-tui', '--format', '--jobs', '--help'
  } elseif ($words[1] -eq 'completion') {
    'bash', 'zsh', 'fish', 'powershell'
  } else { @() }
  $candidates | Where-Object { $_ -like "$wordToComplete*" } |
    ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
`

func cmdCompletion(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gochecks completion <bash|zsh|fish|powershell>")
	}

	scripts := map[string]string{
		"bash":       bashCompletion,
		"zsh":        zshCompletion,
		"fish":       fishCompletion,
		"powershell": powershellCompletion,
	}

	script, ok := scripts[args[0]]
	if !ok {
		return fmt.Errorf("unknown shell %q (want bash, zsh, fish or powershell)", args[0])
	}

	_, err := io.WriteString(stdout, script)

	return err
}
