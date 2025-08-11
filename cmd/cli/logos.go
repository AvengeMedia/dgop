package main

import (
	"os"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

// DistroColors maps distro names to their brand colors
var DistroColors = map[string]string{
	"arch":     "#1793D1", // Arch blue
	"ubuntu":   "#E95420", // Ubuntu orange
	"debian":   "#D70A53", // Debian red
	"fedora":   "#294172", // Fedora blue
	"gentoo":   "#54487A", // Gentoo purple
	"nixos":    "#5277C3", // NixOS blue
	"manjaro":  "#35BF5C", // Manjaro green
	"opensuse": "#73BA25", // openSUSE green
	"centos":   "#EE0000", // CentOS red
	"mint":     "#87CF3E", // Linux Mint green
	"pop":      "#48B9C7", // Pop!_OS teal
	"elementary": "#64BAFF", // elementary blue
}

// Distro logos - high quality ASCII art
var DistroLogos = map[string]string{
	"arch": `       /\        
      /  \       
     /\   \      
    /      \     
   /   ,,   \    
  /   |  |  -\   
 /_-''    ''-_\  `,

	"ubuntu": `         _
     ---(_)
 _/  ---  \
(_) |   |
  \  --- _/
     ---(_)`,

	"debian": `  _____
 /  __ \
|  /    |
|  \___-'
-_
  --_`,

	"fedora": `        ,'''''.           
       |   ,.  |          
       |  |  '_'          
  ,....|  |..               
.'  ,_;|   ..'              
|  |   |  |                 
|  ',_,'  |                 
 '.     ,'                  
   '''''`,

	"gentoo": `         -/oyddmdhs+:.                
     -odNMMMMMMMMNNmhy+-` + "`" + `             
   -yNMMMMMMMMMMMNNNmmdhy+-           
 ` + "`" + `omMMMMMMMMMMMMNmdmmmmddhhy/` + "`" + `       
 omMMMMMMMMMMMNhhyyyohmdddhhhdo` + "`" + `    
.ydMMMMMMMMMMdhs++so/smdddhhhhdm+` + "`" + `  
 oyhdmNMMMMMMMNdyooydmddddhhhhyhNd.  
  :oyhhdNNMMMMMMMNNNmmdddhhhhhyymMh  
    .:+sydNMMMMMNNNmmmdddhhhhhhmMmy  
       /mMMMMMMNNNmmmdddhhhhhmMNhs:  
    ` + "`" + `oNMMMMMMMNNNmmmddddhhdmMNhs+` + "`" + `    
  ` + "`" + `sNMMMMMMMMNNNmmmdddddmNMmhs/.      
 /NMMMMMMMMNNNNmmmdddmNMNdso:` + "`" + `        
+MMMMMMMNNNNNmmmmdmNMNdso/-           
yMMNNNNNNNmmmmmNNMmhs+/` + "`" + `-             
/hmmmmmmmmmmNNNmhs+/` + "`" + `-               
` + "`" + `/ohhhhhhhhhhhs+/` + "`" + `-                 
  ` + "`" + `-//////-` + "`" + ``,

	"nixos": `          ::::.    ` + "`" + `:::::     ::::'          
          ':::::    ` + "`" + `:::::     ::::'          
            :::::     ':::::.  ::::'            
            ':::::       ::::.:::::'            
              :::::.......:::::::::'              
               :::::::::::::::::::                
                ::::::::::::::::                  
           .....::::: ` + "`" + `:::::....             
          ::::::::::'  ` + "`" + `:::::::::            
         :::::::::      :::::::::            
        ::::::::'         '::::::::::         
       .::::::            :::::::::::         
      .::::::           :::::::::::::::....     
     .::::''           '::::::::::::::::::'     
     .:'               '::::::::::::::::'      
    .:                 ` + "`" + `:::::::::::'         
    .                   ` + "`" + `:::::'`,

	"manjaro": `        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
        ████████████████  ████████     
████████████████████████████████████████
████████████████████████████████████████
████████████████████████████████████████
████████████████████████████████████████`,

	"opensuse": `           .;ldkO0000Okdl;.             
        .;d00xl:^''''''^:ok00d;.          
      .d00l'                'o00d.        
    .d0Kd'  Okxol:;,.          :O0d.      
   .OK0yc  .0MMMMMMMMMMcccccc:  :0Ko.     
  :K0kd'   .0MMMMMMMMMMMcccccc.  d0Kk:    
 ;0K0y'    .0MMMMMMMMMMMcccccc   O0Kd;    
.K0kd.     .0MMMMMMMMMMMcccccc.   d0Kk.   
dK0d.      .0MMMMMMMMMMMcccccc     k0Kx   
kK0l       .0MMMMMMMMMMMcccccc     :0Kk   
K0k:       .0MMMMMMMMMMMcccccc     .k0K   
dK0l       .0MMMMMMMMMMMcccccc     :0Kk   
.K0kd.     .0MMMMMMMMMMMcccccc.   d0Kk.   
;0K0y'    .0MMMMMMMMMMMcccccc   O0Kd;    
 :K0kd'   .0MMMMMMMMMMMcccccc.  d0Kk:    
  .OK0yc  .0MMMMMMMMMMMcccccc:  :0Ko.     
   .d0Kd'  ddddddddddddddddd.  :O0d.      
    .d00l'                'o00d.        
      .;d00xl:^''''''^:ok00d;.          
           .;ldkO0000Okdl;.`,

	"mint": `             ...-:::::-...                 
         .-MMMMMMMMMMMMMMM-.              
      .-MMMM` + "`" + `.:(` + "`" + `.-MMMMMMMMM-.            
    .:MMMM.:MMMMMMMMMMMMMMM:.           
   :MMM:MMMMMMMMMMMMMMMMMMM:            
  .MMM.MMMMMMMMMMMMMMMMMMMMM.           
  MMM.MMMMMMMMMMMMMMMMMMMMMMM           
 .MMM.MMMMMMMMMMMMMMMMMMMMMMM.          
 MMM.MMMMMMMMMMMMMMMMMMMMMMMMM          
.MMM.MMMMMMMMMMMMMMMMMMMMMMMMM.         
.MMM.MMMMMMMMMMMMMMMMMMMMMMMMM.         
.MMM.MMMMMMMMMMMMMMMMMMMMMMMMM.         
 MMM.MMMMMMMMMMMMMMMMMMMMMMMMM          
 .MMM.MMMMMMMMMMMMMMMMMMMMMMM.          
  MMM.MMMMMMMMMMMMMMMMMMMMMMM           
  .MMM.MMMMMMMMMMMMMMMMMMMMM.           
   :MMM:MMMMMMMMMMMMMMMMMMM:            
    .:MMMM.:MMMMMMMMMMMMMMM:.           
      .-MMMM` + "`" + `.:(` + "`" + `.-MMMMMMMMM-.            
         .-MMMMMMMMMMMMMMM-.              
             ...-:::::-...`,

	"elementary": `         eeeeeeeeeeeeeeeee            
      eeeeeeeeeeeeeeeeeeeeeee         
    eeeee  eeeeeeeeeeee   eeeee       
  eeee   eeeee       eee     eeee     
 eeee   eeee          eee     eeee    
eee    eee            eee       eee   
eee   eee            eee        eee   
ee    eee           eeee       eeee   
ee    eee         eeeee      eeeeee   
ee    eee       eeeee      eeeee ee   
eee   eeee   eeeeee      eeeee  eee   
eee    eeeeeeeeee     eeeeee    eee   
 eeee    eeeeeeeeeeeeeeeee    eeeee   
  eeeee     eeeeeeeeeeee     eeeeee    
    eeeeeee    eeeeeeee   eeeeeee      
      eeeeeeeeeeeeeeeeeeeeeee         
         eeeeeeeeeeeeeeeee`,
}

// GetDistroLogo returns the logo and color for the current distro
func GetDistroLogo() ([]string, string) {
	// Detect distro from /etc/os-release
	distro := detectDistro()
	
	// Get logo and color
	logo, exists := DistroLogos[distro]
	if !exists {
		// Fallback to generic Linux logo
		logo = `    ___
   (.. \
   (<> |
  //  \\ \
 ( |  | /|
_/\ )  ( /_
\/__\/  \__/`
		distro = "linux"
	}
	
	color, exists := DistroColors[distro]
	if !exists {
		color = "#FFFFFF" // Default white
	}
	
	return formatLogoForTerminal(logo, color), color
}

// formatLogoForTerminal formats the logo with proper width constraints and colors
func formatLogoForTerminal(logo string, color string) []string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	coloredLines := make([]string, 0)
	
	for _, line := range strings.Split(logo, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			// Ensure line isn't too wide (max 40 chars for logos)
			if len(trimmed) > 40 {
				trimmed = trimmed[:40]
			}
			coloredLines = append(coloredLines, style.Render(trimmed))
		}
	}
	
	// Limit to reasonable height
	if len(coloredLines) > 8 {
		coloredLines = coloredLines[:8]
	}
	
	return coloredLines
}

// detectDistro detects the current Linux distribution
func detectDistro() string {
	// Read /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "linux" // fallback
	}
	
	content := strings.ToLower(string(data))
	
	// Check for distro identifiers in order of specificity
	distros := []string{
		"arch", "ubuntu", "debian", "fedora", "gentoo", "nixos", 
		"manjaro", "opensuse", "centos", "mint", "elementary", "pop",
	}
	
	for _, distro := range distros {
		if strings.Contains(content, distro) {
			return distro
		}
	}
	
	return "linux" // fallback
}