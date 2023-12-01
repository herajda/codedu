# code-grader: specification of the idea
 
I want to create an easily to self-host software "code-grader" designed to automatically evaluate and grade coding assignments or projects. It aims to streamline the process of code assessment. A huge inspiration would be our MFF's [ReCodEx](https://github.com/ReCodEx). 

To keep things on the simpler side, the code-grader will grade only Python scripts. This could be easily extended in the future.  


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
- in each class you can see your assignments, their deadlines and possibly their tests
- the student will be allowed to upload their solution





