---
tags: library
description: "Git Tools: Manual Snapshot & History Viewer"
---

# 1. Manual Snapshot
Command: `Git: Snapshot Now`

```space-lua
-- Helper: Run git command in /content
function runGitRaw(args)
  -- Safety: Check for repo
  local check = shell.run("test", {"-d", "/content/.git"})
  if check.code ~= 0 then return nil end

  -- Prepend -C /content
  local gitArgs = {"-C", "/content"}
  for _, arg in ipairs(args) do
    table.insert(gitArgs, arg)
  end

  return shell.run("git", gitArgs)
end

command.define {
  name = "Git: Snapshot Now",
  run = function()
    local status = runGitRaw({"status", "--porcelain"})
    if not status or status.stdout == "" then
      editor.flashNotification("Git: Clean (No changes)")
      return
    end

    runGitRaw({"add", "."})
    runGitRaw({"commit", "-m", "Manual Snapshot: " .. os.date("%Y-%m-%d %H:%M:%S")})
    editor.flashNotification("✅ Snapshot Saved")
  end
}

```

# 2. History Viewer

Command: `Git: View History`
This generates a temporary page with the file's commit log.

```space-lua
command.define {
  name = "Git: View History",
  run = function()
    local page = editor.getCurrentPage()
    local filename = page .. ".md"

    -- 1. Fetch Log
    -- We use a pretty format: Hash - Date - Author : Message
    local res = runGitRaw({
      "log", 
      "-n", "20", 
      "--pretty=format:| `%h` | %ar | **%an** | %s |", 
      "--", 
      filename
    })

    if not res or res.code ~= 0 then
      editor.flashNotification("❌ Error reading history (or file new)")
      return
    end

    if res.stdout == "" then
      editor.flashNotification("ℹ️ No history found for this file.")
      return
    end

    -- 2. Build Markdown
    local md = "# 📜 History: " .. page .. "\n"
    md = md .. "Run `Git: Snapshot Now` to save current changes.\n\n"
    md = md .. "| Commit | Age | Author | Message |\n"
    md = md .. "| :--- | :--- | :--- | :--- |\n"
    md = md .. res.stdout

    -- 3. Write & Navigate
    -- We write to a special "History/" folder which is .gitignored
    local historyPage = "History/" .. page
    space.writePage(historyPage, md)
    editor.navigate(historyPage)
  end
}
