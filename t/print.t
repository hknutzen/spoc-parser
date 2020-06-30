#!/usr/bin/perl

use strict;
use warnings;
use Test::More;
use Test::Differences;
use IPC::Run3;
use File::Temp qw(tempfile);

sub run {
    my ($input) = @_;
    my ($in_fh, $filename) = tempfile(UNLINK => 1);
    print $in_fh $input;
    close $in_fh;
    my $cmd = "./spoc-parser $filename";
    my $stdout;
    my $stderr;
    run3($cmd, \undef, \$stdout, \$stderr);
    my $status = $? >> 8;
    $stderr ||= '';
    $stderr =~ s/\Q$filename\E/INPUT/g;
    return($status, $stdout, $stderr);
}

sub test_run {
    my ($title, $input, $expected) = @_;
    my ($status, $output, $stderr) = run($input);
    if ($status != 0) {
        diag("Unexpected failure:\n$stderr");
        fail($title);
    }
    eq_or_diff("$stderr$output", $expected, $title);
}

sub test_err {
    my ($title, $input, $expected) = @_;
    my ($status, $output, $stderr) = run($input);
    if ($status == 0) {
        diag("Unexpected success\n");
        fail($title);
    }
    $stderr =~ s/Aborted\n$//;
    eq_or_diff($stderr, $expected, $title);
}

my ($title, $in, $out);

############################################################
$title = 'Unknown type';
############################################################

$in = <<'END';
foo:x =
END

$out = <<'END';
Syntax error: Unknown global definition at line 1 of INPUT, near "foo:x<--HERE--> ="
END

test_err($title, $in, $out);

############################################################
$title = 'Empty input';
############################################################

$in = '';

$out = <<'END';
END

test_run($title, $in, $out);

############################################################
$title = 'If only comments in file, they are ignored';
############################################################

$in = <<'END';
# c1
  #c1b

#c2
END

$out = <<'END';
END

test_run($title, $in, $out);

############################################################
$title = 'Empty group';
############################################################

$in = <<'END';
group:g1 = ;
END

$out = <<'END';
group:g1 =
;
END

test_run($title, $in, $out);

############################################################
$title = 'Group with union, intersection, complement';
############################################################

$in = <<'END';
group:g1 =
 host:h1,
 group:g2 & group:g3 &! host:h2 &! host:h3,
 network:n1,;
END

$out = <<'END';
group:g1 =
 group:g2
 & group:g3
 &! host:h2
 &! host:h3
 ,
 network:n1,
 host:h1,
;
END

test_run($title, $in, $out);

############################################################
$title = 'Short automatic groups';
############################################################

$in = <<'END';
group:g1 =
 interface:r1.[auto],
 interface:[
  network:n1
 ].[all],
 interface:r1.[
  auto
 ],
 any:[area:a1]
 ,
 network:[   area:a2, ],  ;
END

$out = <<'END';
group:g1 =
 any:[area:a1],
 network:[area:a2],
 interface:[network:n1].[all],
 interface:r1.[auto],
 interface:r1.[auto],
;
END

test_run($title, $in, $out);

############################################################
$title = 'Nested automatic groups';
############################################################

$in = <<'END';
group:g1 =
 interface:[network:n2, network:n1].[all],
 network:[any:[area:a1]],
 network:[
  interface:[area:a3, area:a2].[all] &! interface:r1.n3, interface:r1.n3.virtual
 ]  ;
END

$out = <<'END';
group:g1 =
 network:[
  any:[area:a1],
 ],
 network:[
  interface:[
   area:a2,
   area:a3,
  ].[all]
  &! interface:r1.n3
  ,
  interface:r1.n3.virtual,
 ],
 interface:[
  network:n1,
  network:n2,
 ].[all],
;
END

test_run($title, $in, $out);

############################################################
$title = 'Simple group with many comments';
############################################################

$in = <<'END';
# head1
# head1a

# head2

# This is g1
group:g1 # g1 trailing
= # g1 trailing2

# g1 post def

description = This is a fine group; # desc
# desc post

# desc post 2

# First element
host:h1, # after first
# post first

# Second
host:h2,; # after second
END

$out = <<'END';
# head1
# head1a

# head2

# This is g1
group:g1 = # g1 trailing
 # g1 trailing2

 # g1 post def

 description = This is a fine group;  # desc

 # desc post

 # desc post 2

 # First element
 host:h1, # after first
 # post first

 # Second
 host:h2, # after second
;
END

test_run($title, $in, $out);

############################################################
$title = 'Ignore comment inside description';
############################################################

$in = <<'END';
group:g1 =
   description # IGNORE
   =  the text; # comment
;
END

$out = <<'END';
group:g1 =
 description =  the text;  # comment

;
END

test_run($title, $in, $out);

############################################################
$title = 'Comment before first element';
############################################################

$in = <<'END';
group:g1 =

 # pre h
 host:h
;
END

$out = <<'END';
group:g1 =
 # pre h
 host:h,
;
END

test_run($title, $in, $out);

############################################################
$title = 'Without trailing comment at end of file';
############################################################

$in = <<'END';
group:g1 =
 host:h # trailing
 , # more trailing
 # and more
;
END

$out = <<'END';
group:g1 =
 host:h, # trailing
;
END

test_run($title, $in, $out);

############################################################
$title = 'Ignore comment in [any|all]';
############################################################

$in = <<'END';
group:g1 =
 interface:r1.[ # trailing
 # pre
auto # trailing2
 # post
], # real trailing
;
END

$out = <<'END';
group:g1 =
 interface:r1.[auto], # real trailing
;
END

test_run($title, $in, $out);

############################################################
$title = 'Ignore comment in short automatic group';
############################################################

$in = <<'END';
group:g1 = network:[ # trailing start
# pre
area:a # trailing
# post
] # real trailing
;
END

$out = <<'END';
group:g1 =
 network:[area:a], # real trailing
;
END

test_run($title, $in, $out);

############################################################
$title = 'Comment in intersection and complement';
############################################################

$in = <<'END';
group:g1 =
 # post g1

 # pre g2
  group:g2 # g2
 # post g2
 & # &
  group:g3 # g3
 &! # &!
  host:h2, # h2
;
END

$out = <<'END';
group:g1 =
 # post g1

 # pre g2
 group:g2 # g2
 # post g2
 # &
 & group:g3 # g3
 # &!
 &! host:h2 # h2
 ,
;
END

test_run($title, $in, $out);

############################################################
$title = 'Comments in automatic group';
############################################################
# trailing start is ignored

$in = <<'END';
group:g1 =
 network:[ #### trailing start
 # pre h1
 host:h1, # trailing h1
 # post h1

 # pre h2
 host:h2
 ,
 host:h3 # trailing h3
] # trailing list
;
END

$out = <<'END';
group:g1 =
 network:[
  # pre h1
  host:h1, # trailing h1
  # post h1

  # pre h2
  host:h2,
  host:h3, # trailing h3
 ], # trailing list
;
END

test_run($title, $in, $out);

############################################################
$title = 'Service with comments';
############################################################

$in = <<'END';
# pre s1
service:s1 = {
 # head s1
 description = s1 # desc s1
 # IGNORED
 user = host:h2, host:h1;
 # pre rule1
 permit src = user; dst = network:n1; prt = tcp 80; # after prt
 # pre rule2
 permit src = network:n2, network:n1;
  # IGNORED
        dst = user;
  # IGNORED
        prt = # pre tcp after '='
           # pre tcp
           tcp 90, # after tcp
           # post tcp
           # pre udp
           udp 123, proto 47, icmp 8; #after icmp
   # Pre log
        log = fw3, # after log1
              asa1; # after log2
}
END

$out = <<'END';
# pre s1
service:s1 = {
 # head s1
 description = s1  # desc s1

 user = host:h1,
        host:h2,
        ;
 # pre rule1
 permit src = user;
        dst = network:n1;
        prt = tcp 80; # after prt
 # pre rule2
 permit src = network:n1,
              network:n2,
              ;
        dst = user;
        prt =
              # pre tcp after '='
              # pre tcp
              tcp 90, # after tcp
              # post tcp
              # pre udp
              udp 123,
              proto 47,
              icmp 8, #after icmp
              ;
        # Pre log
        log = asa1, # after log2
              fw3, # after log1
              ;
}
END

test_run($title, $in, $out);

############################################################
$title = 'Service with attributes';
############################################################

$in = <<'END';
service:s1 = {
 overlaps = service:s3, service:s6, service:s4, service:s2, service:s5;
 multi_owner;
 user = host:h1;
 permit src = user; dst = network:n1; prt = tcp 80;
}
END

$out = <<'END';
service:s1 = {

 multi_owner;
 overlaps = service:s2,
            service:s3,
            service:s4,
            service:s5,
            service:s6,
            ;

 user = host:h1;
 permit src = user;
        dst = network:n1;
        prt = tcp 80;
}
END

test_run($title, $in, $out);

############################################################
$title = 'Service with foreach';
############################################################

$in = <<'END';
service:s1 = {
 user = foreach host:h2, host:h1;
 permit src = user; dst = network:[user]; prt = tcp 80;
}
END

$out = <<'END';
service:s1 = {

 user = foreach
  host:h1,
  host:h2,
 ;
 permit src = user;
        dst = network:[user];
        prt = tcp 80;
}
END

test_run($title, $in, $out);

############################################################
$title = 'Intersection in first line';
############################################################

$in = <<'END';
service:s1 = {
 user = group:g1 &! host:h1;

 permit src = user;
        dst = host:h2;
        prt = tcp 80;
}
END

$out = <<'END';
service:s1 = {

 user = group:g1
        &! host:h1
        ;
 permit src = user;
        dst = host:h2;
        prt = tcp 80;
}
END

test_run($title, $in, $out);

############################################################
$title = 'Automatic group in first line';
############################################################

$in = <<'END';
service:s1 = {
 user = host:[network:n2, network:n1]; # host:h3;
 permit src = user; dst = host:h2; prt = tcp 80;
}
END

$out = <<'END';
service:s1 = {

 user = host:[
         network:n1,
         network:n2,
        ]; # host:h3;
 permit src = user;
        dst = host:h2;
        prt = tcp 80;
}
END

test_run($title, $in, $out);

############################################################
$title = 'Deeply nested automatic group';
############################################################

$in = <<'END';
service:s1 = {
 user = host:[network:[area:a] &! network:[network:[area:b] &! network:n1]];
 permit src = user;
        dst = network:n2;
        prt = tcp 80;
}
END

$out = <<'END';
service:s1 = {

 user = host:[
         network:[area:a]
         &! network:[
             network:[area:b]
             &! network:n1
             ,
            ]
         ,
        ];
 permit src = user;
        dst = network:n2;
        prt = tcp 80;
}
END

test_run($title, $in, $out);

############################################################
done_testing;
