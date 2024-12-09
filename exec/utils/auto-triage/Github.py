from datetime import datetime

class Github:        
    def is_open(self, name):
        #TODO
        return True
    
    def inherit(self, name):
        return {
            "name": name,
            "type": "Github",
            "username": "Cisco InstaTriage",
            "updated": datetime.now(),
            "resolved": not self.is_open(name)
        }
