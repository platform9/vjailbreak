package version

var Version string

func GetVersion() string {
	return `
____   ________________   ____        .__           .__                       
\   \ /   /\_____  \   \ /   /        |  |__   ____ |  | ______   ___________ 
 \   Y   /  /  ____/\   Y   /  ______ |  |  \_/ __ \|  | \____ \_/ __ \_  __ \
  \     /  /       \ \     /  /_____/ |   Y  \  ___/|  |_|  |_> >  ___/|  | \/
   \___/   \_______ \ \___/           |___|  /\___  >____/   __/ \___  >__|   
                   \/                      \/     \/     |__|        \/       
___.                  .__          __    _____                    ________    
\_ |__ ___.__. ______ |  | _____ _/  |__/ ____\___________  _____/   __   \   
 | __ <   |  | \____ \|  | \__  \\   __\   __\/  _ \_  __ \/     \____    /   
 | \_\ \___  | |  |_> >  |__/ __ \|  |  |  | (  <_> )  | \/  Y Y  \ /    /    
 |___  / ____| |   __/|____(____  /__|  |__|  \____/|__|  |__|_|  //____/     
     \/\/      |__|             \/                              \/            

     ` + Version + "\n"
}
