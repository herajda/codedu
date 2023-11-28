#include <boost/asio.hpp>
#include <iostream>
#include <fstream>
#include <sstream>

using namespace boost::asio;
using ip::tcp;
using std::string;
using std::cout;
using std::endl;

string read_file_content(const string& file_path) {
    std::ifstream file(file_path);
    if (!file.is_open()) {
        return "Error: Unable to open file";
    }

    std::stringstream buffer;
    buffer << file.rdbuf();
    file.close();
    return buffer.str();
}

int main() {
    io_service io_service;
    tcp::acceptor acceptor(io_service, tcp::endpoint(tcp::v4(), 8080));

    while (true) {
        tcp::socket socket(io_service);

        acceptor.accept(socket);

        string message = read_file_content("text.txt"); // Replace with your file path
        boost::system::error_code ignored_error;
        write(socket, buffer(message), ignored_error);
    }

    return 0;
}
