#!/usr/bin/env python3
"""
UserPromptSubmit hook for task directory preparation.
Automatically creates task-{id} directories and git branches when users type /task.
"""
import json
import os
import sys
import re
import subprocess
from pathlib import Path


def get_next_task_id(base_dir: Path) -> int:
    """Find the next available task ID by checking existing directories."""
    if not base_dir.exists():
        return 1
    
    existing_dirs = [
        d for d in base_dir.iterdir() 
        if d.is_dir() and d.name.startswith('task-')
    ]
    
    if not existing_dirs:
        return 1
    
    # Extract numbers from directory names
    numbers = []
    for dir_path in existing_dirs:
        match = re.search(r'task-(\d+)', dir_path.name)
        if match:
            numbers.append(int(match.group(1)))
    
    return max(numbers) + 1 if numbers else 1


def create_task_directory(base_dir: Path, task_id: int) -> tuple[bool, str]:
    """Create the task directory and return success status and path."""
    try:
        task_dir = base_dir / f"task-{task_id}"
        
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
    
    # Get next task ID
    base_dir = Path(cwd) / "planning" / "tasks"
    task_id = get_next_task_id(base_dir)
    
    # Create task directory
    success, result = create_task_directory(base_dir, task_id)
    
    if success:
        # Create git branch for this task
        branch_name = f"task-{task_id}"
        branch_created = create_git_branch(branch_name, cwd)
        
        # Extract the original problem from the prompt (after /task)
        problem_text = prompt.replace('/task', '').strip()
        
        # Output context message that will be added to the prompt
        context_msg = f"Directory task-{task_id} has been automatically created for this task session. Git branch 'task-{task_id}' has been created and checked out. The subagents must create the INVESTIGATION_REPORT.md, FLOW_REPORT.md and PLAN.md files inside planning/tasks/task-{task_id}/."
        if not branch_created:
            context_msg += " Note: Git branch creation encountered an issue, but task directory was created."
        if problem_text:
            context_msg += f" Problem to solve: {problem_text}"
        
        print(context_msg)
        sys.exit(0)
    else:
        # Output error but don't block processing
        print(f"Warning: Failed to create task directory: {result}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
