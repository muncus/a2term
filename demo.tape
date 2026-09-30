Output demo.gif

Set FontSize 15
Set Width 1200
Set Height 760
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

# Start a2term connected to the A2A example service on port 9001
Type "./a2term --agent http://localhost:9001"
Enter
Sleep 3s

# Request the full interactive A2UI component showcase
Type "showcase"
Enter
Sleep 3.5s

# Switch to the Surfaces tab to interact with controls
Tab 1
Sleep 2.5s

# Cycle focus to the checkbox
Tab 1
Sleep 1.5s

# Toggle checkbox with Space
Space 1
Sleep 1.5s

# Cycle focus to the theme choice picker
Tab 1
Sleep 1.5s

# Cycle focus to the notes input field
Tab 2
Sleep 1.5s

# Cycle focus to the Submit button
Tab 1
Sleep 1.5s

# Activate Submit button
Enter
Sleep 3s

# Return to Chat feed to view action result
Escape
Sleep 3s

# Display /help command
Type "/help"
Enter
Sleep 3.5s

# Exit cleanly
Type "/quit"
Enter
Sleep 1.5s

