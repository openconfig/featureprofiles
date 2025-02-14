from datetime import datetime

class TechZone:        
    def is_open(self, name):
        """Determine if TechZone Issue is Open. Currently set to always open (always inherit)"""
        return True

    def inherit(self, name):
        """Create a TechZone bug to inherit"""
        return {
            "name": name,
            "type": "TechZone",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name)
        }
