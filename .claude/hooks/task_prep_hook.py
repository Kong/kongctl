#!/usr/bin/env python3
"""
UserPromptSubmit hook for task directory preparation.
Automatically creates task directories and git branches based on the task type:
- GitHub issues: /task gh-XX → ./tasks/gh-XX/
- Planning stages: /task 008-external-resources → ./planning/008-external-resources/tasks/task-XXX/
- Ad-hoc tasks: /task <prompt> → ./tasks/task-XXX/
"""
import json
import os
import sys
import re
import subprocess
from pathlib import Path


def detect_task_mode(task_args: str) -> tuple[str, str]:
    """
    Detect the task mode from the arguments.
    Returns: (mode, identifier) where mode is 'github', 'planning', or 'adhoc'
    """
    task_args = task_args.strip()
    
    # Check for GitHub issue pattern (gh-XX or GH-XX)
    gh_match = re.match(r'^(gh|GH)-(\d+)$', task_args)
    if gh_match:
        return 'github', f"gh-{gh_match.group(2)}"
    
    # Check for planning stage pattern (XXX-name)
    planning_match = re.match(r'^(\d{3}-[\w-]+)$', task_args)
    if planning_match:
        return 'planning', planning_match.group(1)
    
    # Everything else is ad-hoc
    return 'adhoc', task_args


def get_next_task_id(base_dir: Path, prefix: str = 'task-') -> int:
    """Find the next available task ID by checking existing directories."""
    if not base_dir.exists():
        return 1
    
    existing_dirs = [
        d for d in base_dir.iterdir() 
        if d.is_dir() and d.name.startswith(prefix)
    ]
    
    if not existing_dirs:
        return 1
    
    # Extract numbers from directory names
    numbers = []
    for dir_path in existing_dirs:
        match = re.search(rf'{prefix}(\d+)', dir_path.name)
        if match:
            numbers.append(int(match.group(1)))
    
    return max(numbers) + 1 if numbers else 1


def fetch_github_issue(issue_number: str, cwd: str) -> tuple[bool, str]:
    """Fetch GitHub issue details using gh CLI."""
    try:
        # Get issue details
        result = subprocess.run(
            ["gh", "api", f"repos/Kong/kongctl/issues/{issue_number}"],
            cwd=cwd,
            capture_output=True,
            text=True
        )
        
        if result.returncode != 0:
            return False, f"Failed to fetch GitHub issue #{issue_number}: {result.stderr}"
        
        issue_data = json.loads(result.stdout)
        
        # Format issue details for markdown file
        issue_content = f"""# GitHub Issue #{issue_number}

**Title:** {issue_data.get('title', 'No title')}
**State:** {issue_data.get('state', 'unknown')}
**Author:** {issue_data.get('user', {}).get('login', 'unknown')}
**Created:** {issue_data.get('created_at', 'unknown')}
**URL:** {issue_data.get('html_url', '')}

## Description

{issue_data.get('body', 'No description provided')}

## Labels
{', '.join([label['name'] for label in issue_data.get('labels', [])])}

## Additional Context
This issue was automatically fetched from GitHub for task-based resolution.
"""
        
        return True, issue_content
        
    except Exception as e:
        return False, f"Error fetching GitHub issue: {e}"


def get_planning_stage_info(stage_name: str, cwd: str) -> tuple[bool, str, Path]:
    """Get information about a planning stage and verify it exists."""
    planning_dir = Path(cwd) / "planning" / stage_name
    
    if not planning_dir.exists():
        return False, f"Planning stage '{stage_name}' does not exist", None
    
    # Check for execution-plan-steps.md
    steps_file = planning_dir / "execution-plan-steps.md"
    if not steps_file.exists():
        return False, f"No execution-plan-steps.md found for stage '{stage_name}'", None
    
    # Return success with planning directory
    return True, "", planning_dir


def create_task_directory(base_dir: Path, task_name: str, mode: str) -> tuple[bool, str]:
    """Create the task directory based on mode and return success status and path."""
    try:
        task_dir = base_dir / task_name
        
        # Create directories with proper permissions
        base_dir.mkdir(parents=True, exist_ok=True)
        task_dir.mkdir(exist_ok=True)
        
        return True, str(task_dir)
    except Exception as e:
        return False, str(e)


def validate_git_environment(cwd: str) -> tuple[bool, str]:
    """Validate git environment is clean before proceeding with task creation."""
    try:
        # Check if we're in a git repository
        result = subprocess.run(
            ["git", "rev-parse", "--git-dir"], 
            cwd=cwd, 
            capture_output=True, 
            text=True
        )
        if result.returncode != 0:
            return False, "Not in a git repository. Please initialize a git repository first."
        
        # Check for uncommitted changes
        result = subprocess.run(
            ["git", "status", "--porcelain"], 
            cwd=cwd, 
            capture_output=True, 
            text=True
        )
        if result.stdout.strip():
            # Get more detailed status for user
            status_result = subprocess.run(
                ["git", "status", "--short"], 
                cwd=cwd, 
                capture_output=True, 
                text=True
            )
            return False, f"Uncommitted changes detected. Please commit or stash your changes before starting a new task.\n\nCurrent git status:\n{status_result.stdout}"
        
        return True, ""
    except Exception as e:
        return False, f"Failed to check git environment: {e}"


def create_git_branch(branch_name: str, cwd: str) -> bool:
    """Create and checkout a new git branch for the task."""
    try:
        # Check if branch already exists
        result = subprocess.run(
            ["git", "rev-parse", "--verify", f"refs/heads/{branch_name}"], 
            cwd=cwd, 
            capture_output=True, 
            text=True
        )
        if result.returncode == 0:
            # Branch exists, just checkout
            subprocess.run(["git", "checkout", branch_name], cwd=cwd, capture_output=True)
        else:
            # Create and checkout new branch
            subprocess.run(["git", "checkout", "-b", branch_name], cwd=cwd, capture_output=True)
        
        return True
    except Exception as e:
        print(f"Warning: Failed to create git branch: {e}", file=sys.stderr)
        return False


def validate_prompt(prompt: str) -> bool:
    """Check if prompt starts with /task and requires directory setup."""
    # Strip whitespace and check for /task at the start
    cleaned_prompt = prompt.strip()
    return cleaned_prompt.startswith('/task')


def main():
    """Main hook execution logic."""
    try:
        # Read JSON input from stdin
        input_data = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        # Not a JSON input, exit silently
        sys.exit(0)
    
    # Extract required fields
    prompt = input_data.get("prompt", "")
    cwd = input_data.get("cwd", os.getcwd())
    
    # Check if this is a task prompt
    if not validate_prompt(prompt):
        # Not a task prompt, exit silently to allow normal processing
        sys.exit(0)
    
    # Validate git environment before proceeding
    git_valid, git_error = validate_git_environment(cwd)
    if not git_valid:
        print(f"ERROR: Cannot start new task - {git_error}", file=sys.stderr)
        print("\nTo resolve this issue:", file=sys.stderr)
        print("  - Commit your changes: git add . && git commit -m 'your message'", file=sys.stderr)
        print("  - Or stash your changes: git stash", file=sys.stderr)
        print("  - Or discard changes: git checkout .", file=sys.stderr)
        sys.exit(2)  # Exit with error code to block the task
    
    # Extract the task arguments (after /task)
    task_args = prompt.replace('/task', '').strip()
    
    # Detect task mode
    mode, identifier = detect_task_mode(task_args)
    
    # Handle based on mode
    if mode == 'github':
        # GitHub issue mode
        issue_number = identifier.replace('gh-', '')
        base_dir = Path(cwd) / "planning" / "tasks"
        task_dir_name = identifier
        branch_name = identifier
        
        # Fetch GitHub issue
        success, issue_content = fetch_github_issue(issue_number, cwd)
        if not success:
            print(f"ERROR: {issue_content}", file=sys.stderr)
            sys.exit(2)
        
        # Create task directory
        success, task_dir = create_task_directory(base_dir, task_dir_name, mode)
        if not success:
            print(f"ERROR: Failed to create task directory: {task_dir}", file=sys.stderr)
            sys.exit(2)
        
        # Write GitHub issue details to file
        issue_file = Path(task_dir) / "GITHUB_ISSUE.md"
        issue_file.write_text(issue_content)
        
        context_msg = f"GitHub issue #{issue_number} task directory has been created at planning/tasks/{identifier}/. Git branch '{branch_name}' has been created and checked out. The issue details have been saved to GITHUB_ISSUE.md. The subagents must create the INVESTIGATION_REPORT.md, FLOW_REPORT.md and PLAN.md files inside planning/tasks/{identifier}/."
        
    elif mode == 'planning':
        # Planning stage mode
        stage_name = identifier
        valid, error, planning_dir = get_planning_stage_info(stage_name, cwd)
        if not valid:
            print(f"ERROR: {error}", file=sys.stderr)
            sys.exit(2)
        
        # Get next task ID for this planning stage
        tasks_dir = planning_dir / "tasks"
        task_id = get_next_task_id(tasks_dir, 'task-')
        task_dir_name = f"task-{task_id}"
        branch_name = f"{stage_name}-task-{task_id}"
        
        # Create task directory
        success, task_dir = create_task_directory(tasks_dir, task_dir_name, mode)
        if not success:
            print(f"ERROR: Failed to create task directory: {task_dir}", file=sys.stderr)
            sys.exit(2)
        
        context_msg = f"Planning stage '{stage_name}' task directory has been created at planning/{stage_name}/tasks/{task_dir_name}/. Git branch '{branch_name}' has been created and checked out. This task is for implementing the next step in the {stage_name} planning stage. The subagents must create the INVESTIGATION_REPORT.md, FLOW_REPORT.md and PLAN.md files inside planning/{stage_name}/tasks/{task_dir_name}/. Reference the execution-plan-steps.md in planning/{stage_name}/ for implementation context."
        
    else:
        # Ad-hoc task mode
        base_dir = Path(cwd) / "planning" / "tasks"
        task_id = get_next_task_id(base_dir, 'task-')
        task_dir_name = f"task-{task_id}"
        branch_name = task_dir_name
        
        # Create task directory
        success, task_dir = create_task_directory(base_dir, task_dir_name, mode)
        if not success:
            print(f"ERROR: Failed to create task directory: {task_dir}", file=sys.stderr)
            sys.exit(2)
        
        context_msg = f"Directory {task_dir_name} has been automatically created for this task session at planning/tasks/{task_dir_name}/. Git branch '{branch_name}' has been created and checked out. The subagents must create the INVESTIGATION_REPORT.md, FLOW_REPORT.md and PLAN.md files inside planning/tasks/{task_dir_name}/."
        if task_args:
            context_msg += f" Problem to solve: {task_args}"
    
    # Create git branch
    branch_created = create_git_branch(branch_name, cwd)
    if not branch_created:
        context_msg += " Note: Git branch creation encountered an issue, but task directory was created."
    
    print(context_msg)
    sys.exit(0)


if __name__ == "__main__":
    main()