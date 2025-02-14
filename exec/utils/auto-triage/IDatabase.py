from abc import ABC, abstractmethod

class IDatabase(ABC):
    """Implement abstract class to handle database functions"""
    @abstractmethod
    def insert_logs(self, documents):
        pass

    @abstractmethod
    def insert_metadata(self, document={}):
        pass

    @abstractmethod
    def get_datapoints(self):
        pass

    @abstractmethod
    def is_subscribed(self, name):
        pass

    @abstractmethod
    def get_historical_testsuite(self, lineup, group, plan):
        pass