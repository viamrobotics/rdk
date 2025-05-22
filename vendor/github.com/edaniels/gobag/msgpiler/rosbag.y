%{
package msgpiler

%}

%union {
   identifierValue string
   typeValue string
   integerValue int
}

%token <identifierValue> IDENTIFIER
%token <typeValue> SIMPLETYPE
%token <integerValue> INTEGER

%token COMMENT CONSTANT NEWLINE LEFTBRACKET RIGHTBRACKET MSGDEF NAMESPACESEPARATOR

%%

lines
        : lines line
        | 
        ;

line
        : definitionline | commentline | constantline | msgdefline | emptyline;

definitionline
        : datatype variable constantopt commentopt NEWLINE
        {
                builder.processDefinition();
        };

datatype
        : basetype | complextype
        ;
;

basetype 
        : SIMPLETYPE arrayopt
        {
                builder.processBaseType($1);
        };

arrayopt
        : arrayoptpresent
        | 
        ;

arrayoptpresent
        : LEFTBRACKET integeropt RIGHTBRACKET 
        {
                builder.processArray();
        };

integeropt
        : arraylength
        | 
        ;

arraylength
        : INTEGER 
        {
                builder.processArrayLength($1);
        };

complextype 
        : complextypename arrayopt
        | namespace complextypename arrayopt

complextypename
        : IDENTIFIER
        {
                builder.processComplexType($1);
        };        

variable
        :IDENTIFIER
        {
                builder.processVariable($1);
        };
        
namespace 
        : namespacestep
        | namespace namespacestep
        ;

namespacestep
        : IDENTIFIER NAMESPACESEPARATOR
        {
                builder.processNamespace($1);
        };        

commentopt
        : COMMENT
        |
        ;

constantopt
        : constantoptpresent
        |
        ;

constantoptpresent
        : CONSTANT
        {
                builder.processConstant();
        };

commentline
        : COMMENT NEWLINE
        ;

constantline
        : CONSTANT NEWLINE
        ;
        
msgdefline
        : MSGDEF complextype commentopt NEWLINE
        {
                builder.processMessageDefinition();
        };
        
emptyline
        : NEWLINE
        ;
