from time import time

from couchbase.cluster import Cluster, ClusterOptions
from couchbase_core.cluster import PasswordAuthenticator
from couchbase.exceptions import DocumentExistsException, \
    DocumentNotFoundException, CouchbaseException
import couchbase.subdocument as sub_doc
from services.profile.utils.constants import Queries


class CBConnection:
    def __init__(self, username, password, host):
        self.cluster = Cluster("couchbase://{0}?ssl=no_verify".format(host),
                               ClusterOptions(PasswordAuthenticator(
                                   username, password)))

        self.cb = self.cluster.bucket("e2e")
        self.cb_coll = self.cb.scope("profiles").collection("users")

    def add_user(self, firstname, lastname, username, password, pass_id):
        # logger.info("Inserting Document")
        doc = {"firstname": firstname, "lastname": lastname,
               "username": username, "password": password,
               "bookings": [], "id": pass_id}
        try:
            self.cb_coll.insert(f'{firstname}_{lastname}', doc)
            return True
        except DocumentExistsException:
            return False

    def delete_user(self, firstname, lastname, user_id_hash):
        try:
            # Remove the document from profiles::users
            self.cb_coll.remove(f'{firstname}_{lastname}')
            # Remove the document from profiles::wallet
            wallet = self.cb.scope("profiles").collection("wallet")
            wallet.remove(user_id_hash)
        except DocumentNotFoundException:
            pass
        return True

    def create_wallet(self, doc_id, name_on_card, card_num, card_cvv,
                      card_expiry, user_id_hash):
        wallet = self.cb.scope("profiles").collection("wallet")
        doc = {"docId": doc_id, "name_on_card": name_on_card,
               "card_number": card_num, "cvv": card_cvv,
               "recharge_history": [], "wallet_balance": 0,
               "expiry": card_expiry, "id": user_id_hash}
        try:
            wallet.insert(user_id_hash, doc)
        except CouchbaseException as e:
            print("CB EXCEPTION: %s" % e)
            return False
        return True

    def load_wallet(self, doc_id, amt_to_load):
        wallet = self.cb.scope("profiles").collection("wallet")
        try:
            amt = wallet.lookup_in(doc_id, [sub_doc.get("wallet_balance")]) \
                .content_as[float](0)
            amt += float(amt_to_load)
            wallet.mutate_in(doc_id,
                             (sub_doc.upsert("wallet_balance", amt),
                              sub_doc.array_append("recharge_history",
                                                   [time(), amt_to_load])))
        except CouchbaseException as e:
            print("CB EXCEPTION: %s" % e)
            return False
        except Exception as e:
            print("Generic exception %s" % e)
            return False
        return True

    def create_profile_users_primary_index(self):
        query = "CREATE PRIMARY INDEX profile_users_pri_index " \
                "IF NOT EXISTS ON `e2e`.`profiles`.`users`"
        try:
            self.run_query(query)
        except CouchbaseException:
            pass

    def get_user(self, user):
        query = Queries.get_user_password.format(user)
        return self.run_query(query)

    def get_user_id(self, user):
        query = Queries.get_user_id.format(user)
        return self.run_query(query)

    def get_all_bookings(self, user):
        query = Queries.get_all_bookings.format(user)
        return self.run_query(query)

    def run_query(self, query):
        res = self.cb.query(query)
        result_arr = [x for x in res]
        print(result_arr)
        return result_arr

    def update_user(self, username, booking_id):
        query = Queries.update_bookings.format(booking_id, username)
        _ = self.run_query(query)

    def get_api_details(self, service, uri):
        api_details = None
        query = Queries.get_api_details.format(service, uri)
        print(query)
        res = self.cb.query(query)
        print(res)
        result_arr = [x for x in res]
        if len(result_arr) != 0:
            api_details = result_arr[0]
        return api_details
