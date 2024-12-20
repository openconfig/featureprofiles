from datetime import datetime

class Github:        
    def is_open(self, name):
        """Determine if Github Issue is Open. Currently set to always open (always inherit)"""
        return True
    
    def inherit(self, name):
        """Create a Github bug to inherit"""
        return {
            "name": name,
            "type": "Github",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name)
        }
