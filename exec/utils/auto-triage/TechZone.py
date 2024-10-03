from datetime import datetime

class TechZone:        
    def inherit(self, name):
        return {
            "name": name,
            "type": "TechZone",
            "username": "Cisco InstaTriage",
            "updated": datetime.now()
        }
