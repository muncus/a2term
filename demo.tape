Output demo.gif

Set FontSize 15
Set Width 1200
Set Height 760
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

# Start a2term connected to the A2A example service on port 9001
Type "./a2term --agent http://localhost:9001"
Enter
Sleep 2s

# Request interactive buttons card
Type "buttons"
Enter
Sleep 2.5s

# Switch keyboard focus to the A2UI surface (or switch to Surfaces tab)
Tab 1
Sleep 1.5s

# Navigate through surface buttons using Tab
Tab 1
Sleep 800ms
Tab 1
Sleep 800ms

# Activate selected button (Enter)
Enter
Sleep 2s

# Return focus back to chat feed
Escape
Sleep 1s

# Request multi-part response with text and interactive A2UI
Type "multipart"
Enter
Sleep 2.5s

# Switch to Logs & Diagnostics tab using slash command
Type "/tab logs"
Enter
Sleep 2s

# Return to Chat feed
Escape
Sleep 1s

# Display /help command showcasing tabs and keybindings
Type "/help"
Enter
Sleep 3s

# Exit cleanly
Type "/quit"
Enter
Sleep 1s
