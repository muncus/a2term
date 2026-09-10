Output demo.gif

Set FontSize 15
Set Width 1200
Set Height 760
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

# Start a2term connected to the running A2A agent on port 9002
Type "./a2term --agent http://localhost:9002"
Enter
Sleep 2s

# Request interactive buttons card
Type "buttons"
Enter
Sleep 2s

# Switch keyboard focus to the A2UI surface
Tab 1
Sleep 1s

# Navigate through surface buttons using Tab / Arrow keys
Tab 1
Sleep 800ms
Tab 1
Sleep 800ms

# Activate selected button (Enter)
Enter
Sleep 2.5s

# Request weather widget
Type "weather"
Enter
Sleep 2s

# Focus the weather surface and refresh
Tab 1
Sleep 800ms
Enter
Sleep 2.5s

# Return focus back to chat input
Escape
Sleep 1s

# Display /help command
Type "/help"
Enter
Sleep 3s

# Exit cleanly
Type "/quit"
Enter
Sleep 1s
