# code-grader: specification of the idea
 
I want to create an easily to self-host software "code-grader" designed to automatically evaluate and grade coding assignments or projects. It aims to streamline the process of code assessment. A huge inspiration would be our MFF's [ReCodEx](https://github.com/ReCodEx). 

To keep things on the simpler side, the code-grader will grade only Python scripts. This could be easily extended in the future.  

### Overview of the program

The first thing you see, when you open the web app is a login page. There you'll have a chance to log in as a student or a teacher (possibly admin). Or create an account, but only for students, the teachers have to be added by admin. 

#### Create an account 
- only for students 
- form with
  - first name
  - last name
  - nickname
  - (no email, that would have to work with a email server, which we don't have)

#### When you log in as a teacher: 
You have several options to do:

- manage classes and add students in them
- add assignment to a class and specify it - name of the assignment, what the student should do
  - for each assignment the teacher can
    - add tests; each test have these parameters
      - input (stdin)
      - required output (stdout)
      - input visibility to student - default FALSE
      - (if I have time: max time, memory)
    - say how the assignments should be graded - success/fail - percentage of tests correct/all tests have to be correct 
    - specify the deadline (for simplicity only one)


#### When you log in as a student

You have several options to do:

- see in which classes you are added
- in each class the student can see their assignments, deadlines and tests
- the student will be allowed to upload their solution for each assignment and see how the tests go 


### Technical specification

The code-grader will have frontend developed using HTML/CSS + React and backend in C++.

I'll use [Crow](https://crowcpp.org/master/) and [Boost.Asio](https://www.boost.org/doc/libs/1_78_0/doc/html/boost_asio.html) for communicating between the backend server and the frontend website. I am also considering using [Oat++](https://oatpp.io/).

A big technical question is how the test will be run. To ensure safety, all tests will evoke creating a Docker container just for the purpose of running one test. 
This could hopefully be done using simple system calls.





