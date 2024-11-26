from datetime import datetime

class Github:        
    def inherit(self, name):
        return {
            "name": name,
            "type": "Github",
            "username": "Cisco InstaTriage",
            "updated": datetime.now()
        }
